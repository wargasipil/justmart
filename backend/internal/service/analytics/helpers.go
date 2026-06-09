package analytics

import (
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	analyticsifacev1 "github.com/justmart/backend/gen/analytics_iface/v1"
)

// analyticsDateRange defaults to the last 30 days when either bound is unset.
func analyticsDateRange(f *analyticsifacev1.Filter) (time.Time, time.Time) {
	now := time.Now()
	var from, to time.Time
	if f != nil && f.ToUnix > 0 {
		to = time.Unix(f.ToUnix, 0)
	} else {
		to = now
	}
	if f != nil && f.FromUnix > 0 {
		from = time.Unix(f.FromUnix, 0)
	} else {
		from = to.AddDate(0, 0, -30)
	}
	return from, to
}

// truncFmt maps Granularity to the bucket string used by the Go helpers.
func truncFmt(g analyticsifacev1.Granularity) string {
	switch g {
	case analyticsifacev1.Granularity_GRANULARITY_MONTH:
		return "month"
	case analyticsifacev1.Granularity_GRANULARITY_WEEK:
		return "week"
	default:
		return "day"
	}
}

// enumerateBuckets walks [from, to) in granularity-sized steps in local time,
// returning the bucket-start timestamp for each step.
func enumerateBuckets(from, to time.Time, granularity string) []time.Time {
	out := []time.Time{}
	cur := bucketStart(from, granularity)
	for cur.Before(to) {
		out = append(out, cur)
		cur = bucketNext(cur, granularity)
	}
	return out
}

// bucketStart truncates t to the start of its bucket.
func bucketStart(t time.Time, granularity string) time.Time {
	loc := t.Location()
	switch granularity {
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	case "week":
		// ISO week starts Monday. Go's Weekday: Sunday=0..Saturday=6.
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		return d.AddDate(0, 0, -(wd - 1))
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
}

// bucketNext returns the next bucket's start.
func bucketNext(t time.Time, granularity string) time.Time {
	switch granularity {
	case "month":
		return t.AddDate(0, 1, 0)
	case "week":
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// dayBucketKey formats a bucket boundary timestamp into the response key.
func dayBucketKey(t time.Time, granularity string) string {
	switch granularity {
	case "month":
		return t.Format("2006-01")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	default:
		return t.Format("2006-01-02")
	}
}

// containsMetric reports whether the metric types list includes the given type.
func containsMetric(types []analyticsifacev1.MetricType, want analyticsifacev1.MetricType) bool {
	for _, m := range types {
		if m == want {
			return true
		}
	}
	return false
}

// validateMetricTypes rejects duplicates, _UNSPECIFIED, and the forbidden type.
func validateMetricTypes(types []analyticsifacev1.MetricType, forbid analyticsifacev1.MetricType, forbidMsg string) error {
	if len(types) == 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("metric_types must not be empty"))
	}
	seen := map[analyticsifacev1.MetricType]bool{}
	for _, m := range types {
		if m == analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("metric_types must not contain UNSPECIFIED"))
		}
		if seen[m] {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("metric_types must not contain duplicates"))
		}
		seen[m] = true
		if forbid != analyticsifacev1.MetricType_METRIC_TYPE_UNSPECIFIED && m == forbid {
			return connect.NewError(connect.CodeInvalidArgument, errors.New(forbidMsg))
		}
	}
	return nil
}

// validateSort enforces "Sort.field must reference a requested metric type".
func validateSort(sort *analyticsifacev1.Sort, types []analyticsifacev1.MetricType, forbidStock bool) error {
	if sort == nil {
		return nil
	}
	switch sort.Field.(type) {
	case *analyticsifacev1.Sort_Order:
		if !containsMetric(types, analyticsifacev1.MetricType_METRIC_TYPE_ORDER) {
			return connect.NewError(connect.CodeInvalidArgument,
				errors.New("sort field references ORDER but metric_types does not include METRIC_TYPE_ORDER"))
		}
	case *analyticsifacev1.Sort_Stock:
		if forbidStock {
			return connect.NewError(connect.CodeInvalidArgument,
				errors.New("stock sort not supported on user dimension"))
		}
		if !containsMetric(types, analyticsifacev1.MetricType_METRIC_TYPE_STOCK) {
			return connect.NewError(connect.CodeInvalidArgument,
				errors.New("sort field references STOCK but metric_types does not include METRIC_TYPE_STOCK"))
		}
	}
	return nil
}

// sortDirSQL maps SortDirection to "ASC"/"DESC" (default ASC).
func sortDirSQL(d analyticsifacev1.SortDirection) string {
	if d == analyticsifacev1.SortDirection_SORT_DIRECTION_DESC {
		return "DESC"
	}
	return "ASC"
}

// orderMetricColumn maps an OrderMetricField to its SQL column alias.
func orderMetricColumn(f analyticsifacev1.OrderMetricField) string {
	switch f {
	case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_HPP:
		return "hpp"
	case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_PROFIT:
		return "profit"
	case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_LAST_ORDER:
		return "last_order_unix"
	case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_AVG_SOLD:
		return "avg_sold"
	default:
		return "terjual"
	}
}

func stockMetricColumn(f analyticsifacev1.StockMetricField) string {
	switch f {
	case analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_ONGOING:
		return "ongoing"
	case analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_LAST_RESTOCK:
		return "last_restock_unix"
	case analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_EXPIRING:
		return "expiring"
	default:
		return "ready"
	}
}

// sortDailyKeys returns the day-keys in the order the UI should render.
func sortDailyKeys(
	keys []string,
	sort *analyticsifacev1.Sort,
	order map[string]*analyticsifacev1.OrderItem,
	stock map[string]*analyticsifacev1.StockItem,
) []string {
	if sort == nil || sort.Field == nil {
		if sort != nil && sort.Direction == analyticsifacev1.SortDirection_SORT_DIRECTION_DESC {
			out := make([]string, len(keys))
			for i, k := range keys {
				out[len(keys)-1-i] = k
			}
			return out
		}
		return keys
	}
	out := make([]string, len(keys))
	copy(out, keys)
	desc := sort.Direction == analyticsifacev1.SortDirection_SORT_DIRECTION_DESC
	switch f := sort.Field.(type) {
	case *analyticsifacev1.Sort_Order:
		col := f.Order
		sortInPlace(out, desc, func(k string) int64 {
			o := order[k]
			if o == nil {
				return 0
			}
			switch col {
			case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_HPP:
				return o.Hpp
			case analyticsifacev1.OrderMetricField_ORDER_METRIC_FIELD_PROFIT:
				return o.Profit
			default:
				return o.Terjual
			}
		})
	case *analyticsifacev1.Sort_Stock:
		col := f.Stock
		sortInPlace(out, desc, func(k string) int64 {
			s := stock[k]
			if s == nil {
				return 0
			}
			if col == analyticsifacev1.StockMetricField_STOCK_METRIC_FIELD_ONGOING {
				return s.Ongoing
			}
			return s.Ready
		})
	}
	return out
}

// sortInPlace sorts the string slice by the int64-valued accessor, asc or desc.
func sortInPlace(xs []string, desc bool, val func(string) int64) {
	for i := 1; i < len(xs); i++ {
		x := xs[i]
		vx := val(x)
		j := i - 1
		for j >= 0 {
			cmp := val(xs[j]) < vx
			if desc {
				cmp = val(xs[j]) > vx
			}
			if cmp {
				break
			}
			xs[j+1] = xs[j]
			j--
		}
		xs[j+1] = x
	}
}

func ensureOrderForKeys(m map[string]*analyticsifacev1.OrderItem, keys []string) map[string]*analyticsifacev1.OrderItem {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			m[k] = &analyticsifacev1.OrderItem{}
		}
	}
	return m
}

func ensureStockForKeys(m map[string]*analyticsifacev1.StockItem, keys []string) map[string]*analyticsifacev1.StockItem {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			m[k] = &analyticsifacev1.StockItem{}
		}
	}
	return m
}
