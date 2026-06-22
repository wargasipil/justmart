package sale

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/service/common"
)

// GetMyPerformance returns the CALLER's own COMPLETED-sales metrics bucketed
// over a date range. It is ALWAYS self-scoped from the principal (there is no
// cashier param) and carries no profit/COGS, so it can never leak cost basis or
// another cashier's data. Revenue/items mirror GetSalesSummary/GetTodaySnapshot
// (SUM(sales.total) / SUM(sale_items.base_qty)) so the numbers reconcile.
func (s *SaleService) GetMyPerformance(
	ctx context.Context,
	req *connect.Request[posifacev1.GetMyPerformanceRequest],
) (*connect.Response[posifacev1.GetMyPerformanceResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	cashierID := caller.UserID // always self — no request field to widen

	from, to := perfDateRange(req.Msg.FromUnix, req.Msg.ToUnix)
	gran := perfGranularity(req.Msg.Granularity)

	// Ordered, zero-filled buckets for the whole range.
	bucketStarts := perfEnumerateBuckets(from, to, gran)
	orderedKeys := make([]string, 0, len(bucketStarts))
	byKey := make(map[string]*posifacev1.PerformanceBucket, len(bucketStarts))
	for _, b := range bucketStarts {
		key := perfDayKey(b, gran)
		orderedKeys = append(orderedKeys, key)
		byKey[key] = &posifacev1.PerformanceBucket{DayKey: key}
	}
	if len(orderedKeys) == 0 {
		return connect.NewResponse(&posifacev1.GetMyPerformanceResponse{}), nil
	}

	// foldKey maps a per-day SQL row ("YYYY-MM-DD") into its bucket key.
	foldKey := func(day string) (string, bool) {
		t, perr := time.ParseInLocation("2006-01-02", day, time.Local)
		if perr != nil {
			return "", false
		}
		key := perfDayKey(perfBucketStart(t, gran), gran)
		_, ok := byKey[key]
		return key, ok
	}

	// Revenue + sale count per day.
	type revRow struct {
		Day     string `gorm:"column:day"`
		Revenue int64  `gorm:"column:revenue"`
		Cnt     int64  `gorm:"column:cnt"`
	}
	var revRows []revRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT `+common.DayKeyExpr(s.db, "completed_at")+` AS day,
		       COALESCE(SUM(total), 0) AS revenue,
		       COUNT(*) AS cnt
		FROM sales
		WHERE status = ? AND warehouse_id = ? AND cashier_user_id = ?
		  AND completed_at >= ? AND completed_at < ?
		GROUP BY day
	`, saleStatusCompleted, warehouseID, cashierID, from, to).Scan(&revRows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range revRows {
		if key, ok := foldKey(r.Day); ok {
			byKey[key].Revenue += r.Revenue
			byKey[key].SalesCount += r.Cnt
		}
	}

	// Items sold (base units) per day.
	type itemRow struct {
		Day   string `gorm:"column:day"`
		Items int64  `gorm:"column:items"`
	}
	var itemRows []itemRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT `+common.DayKeyExpr(s.db, "s.completed_at")+` AS day,
		       COALESCE(SUM(si.base_qty), 0) AS items
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		WHERE s.status = ? AND s.warehouse_id = ? AND s.cashier_user_id = ?
		  AND s.completed_at >= ? AND s.completed_at < ?
		GROUP BY day
	`, saleStatusCompleted, warehouseID, cashierID, from, to).Scan(&itemRows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range itemRows {
		if key, ok := foldKey(r.Day); ok {
			byKey[key].ItemsSold += r.Items
		}
	}

	resp := &posifacev1.GetMyPerformanceResponse{
		Buckets: make([]*posifacev1.PerformanceBucket, 0, len(orderedKeys)),
	}
	for _, key := range orderedKeys {
		b := byKey[key]
		resp.Buckets = append(resp.Buckets, b)
		resp.TotalRevenue += b.Revenue
		resp.TotalSalesCount += b.SalesCount
		resp.TotalItemsSold += b.ItemsSold
	}
	return connect.NewResponse(resp), nil
}

// perfDateRange defaults to the last 30 days when either bound is unset.
func perfDateRange(fromUnix, toUnix int64) (time.Time, time.Time) {
	now := time.Now()
	to := now
	if toUnix > 0 {
		to = time.Unix(toUnix, 0)
	}
	from := to.AddDate(0, 0, -30)
	if fromUnix > 0 {
		from = time.Unix(fromUnix, 0)
	}
	return from, to
}

// perfGranularity maps the proto enum to the bucket-step string.
func perfGranularity(g posifacev1.PerformanceGranularity) string {
	switch g {
	case posifacev1.PerformanceGranularity_PERFORMANCE_GRANULARITY_MONTH:
		return "month"
	case posifacev1.PerformanceGranularity_PERFORMANCE_GRANULARITY_WEEK:
		return "week"
	default:
		return "day"
	}
}

// perfEnumerateBuckets walks [from, to) in granularity-sized steps in local time.
func perfEnumerateBuckets(from, to time.Time, gran string) []time.Time {
	out := []time.Time{}
	cur := perfBucketStart(from, gran)
	for cur.Before(to) {
		out = append(out, cur)
		cur = perfBucketNext(cur, gran)
	}
	return out
}

// perfBucketStart truncates t to the start of its bucket (week = ISO Monday).
func perfBucketStart(t time.Time, gran string) time.Time {
	loc := t.Location()
	switch gran {
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	case "week":
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7 // Go: Sunday=0; ISO week starts Monday
		}
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		return d.AddDate(0, 0, -(wd - 1))
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
}

func perfBucketNext(t time.Time, gran string) time.Time {
	switch gran {
	case "month":
		return t.AddDate(0, 1, 0)
	case "week":
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// perfDayKey formats a bucket-start timestamp into the response key.
func perfDayKey(t time.Time, gran string) string {
	switch gran {
	case "month":
		return t.Format("2006-01")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	default:
		return t.Format("2006-01-02")
	}
}
