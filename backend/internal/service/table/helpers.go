package table

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// maxTableCodeLen bounds the floor label. It renders inside a fixed-size tile on
// the floor plan and on a 32-column thermal ticket, so an unbounded string just
// overflows both.
const maxTableCodeLen = 16

func (s *TableService) load(ctx context.Context, id string) (*model.DiningTable, error) {
	if strings.TrimSpace(id) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var t model.DiningTable
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("table %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &t, nil
}

func toProto(t *model.DiningTable) *tableifacev1.DiningTable {
	return &tableifacev1.DiningTable{
		Id:          t.ID,
		WarehouseId: t.WarehouseID,
		Code:        t.Code,
		Name:        t.Name,
		Area:        t.Area,
		Seats:       t.Seats,
		Active:      t.Active,
		CreatedAt:   t.CreatedAt.Unix(),
	}
}

func validateTable(code string, seats int32) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", common.TokenError(connect.CodeInvalidArgument, "table.code_required")
	}
	if len([]rune(code)) > maxTableCodeLen {
		return "", common.TokenError(connect.CodeInvalidArgument, "table.code_too_long")
	}
	if seats < 0 {
		return "", common.TokenError(connect.CodeInvalidArgument, "table.seats_invalid")
	}
	return code, nil
}

// assertCodeFree pre-checks the partial unique index so the caller gets a
// specific token instead of a raw constraint violation. Scoped to ACTIVE tables
// in the same outlet, matching the index: an archived "T1" must not block
// reusing the number when the floor is rearranged.
func assertCodeFree(db *gorm.DB, warehouseID, code, excludeID string) error {
	q := db.Model(&model.DiningTable{}).
		Where("warehouse_id = ? AND code = ? AND active", warehouseID, code)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if n > 0 {
		return common.TokenError(connect.CodeAlreadyExists, "table.code_taken")
	}
	return nil
}

// openBill is the DRAFT sale currently seated at a table, if any.
type openBill struct {
	SaleID     string `gorm:"column:id"`
	OpenedAt   int64  `gorm:"column:opened_at"`
	Total      int64  `gorm:"column:total"`
	ItemCount  int32  `gorm:"column:item_count"`
	GuestCount int32  `gorm:"column:guest_count"`
	CashierID  string `gorm:"column:cashier_user_id"`
	TableID    string `gorm:"column:table_id"`
}

// loadOpenBills returns the open bill per table id, in ONE query with a grouped
// item count (no N+1 over a floor of tables). Tables with no DRAFT sale are
// simply absent.
//
// The DRAFT filter is what makes this correct: a completed or voided bill leaves
// the table free, so occupancy needs no cleanup step and cannot get stuck.
func loadOpenBills(ctx context.Context, db *gorm.DB, tableIDs []string) (map[string]openBill, error) {
	out := map[string]openBill{}
	if len(tableIDs) == 0 {
		return out, nil
	}
	var rows []openBill
	if err := db.WithContext(ctx).
		Table("sales AS s").
		Select("s.id AS id, s.table_id AS table_id, s.total AS total, "+
			"s.guest_count AS guest_count, s.cashier_user_id AS cashier_user_id, "+
			common.EpochExpr(db, "s.created_at")+" AS opened_at, "+
			"(SELECT COUNT(*) FROM sale_items si WHERE si.sale_id = s.id) AS item_count").
		Where("s.table_id IN ? AND s.status = ?", tableIDs, common.SaleStatusDraft).
		Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range rows {
		out[r.TableID] = r
	}
	return out, nil
}

// applyOccupancy copies an open bill onto its table proto. Kept separate from
// loadOpenBills so both the list and the single-table reads share one definition
// of what "occupied" puts on the wire.
func applyOccupancy(t *tableifacev1.DiningTable, bills map[string]openBill) {
	b, ok := bills[t.Id]
	if !ok {
		return
	}
	t.OpenSaleId = b.SaleID
	t.OpenedAt = b.OpenedAt
	t.OpenTotal = b.Total
	t.OpenItemCount = b.ItemCount
	t.GuestCount = b.GuestCount
	t.OpenedByUserId = b.CashierID
}
