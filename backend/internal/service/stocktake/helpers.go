package stocktake

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// addBatches snapshots expected_qty for each batch and inserts lines.
// Existing (session_id, batch_id) pairs are silently skipped.
func (s *StocktakeService) addBatches(
	ctx context.Context,
	sessionID string,
	batchIDs []string,
) (added int32, skipped int32, err error) {
	if sessionID == "" {
		return 0, 0, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id required"))
	}
	if len(batchIDs) == 0 {
		return 0, 0, nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess, ierr := lockDraftSession(tx, sessionID)
		if ierr != nil {
			return ierr
		}
		whID := common.Deref(sess.WarehouseID)
		for _, bid := range batchIDs {
			// expected_qty is snapshotted per the session's warehouse.
			qty, ierr := common.BatchQtyInWarehouse(ctx, tx, bid, whID)
			if ierr != nil {
				return connect.NewError(connect.CodeInternal, ierr)
			}
			line := model.StocktakeLine{
				SessionID:   sessionID,
				BatchID:     bid,
				ExpectedQty: int32(qty),
				Disposition: dispositionAdjustment,
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&line)
			if res.Error != nil {
				return connect.NewError(connect.CodeInternal, res.Error)
			}
			if res.RowsAffected > 0 {
				added++
			} else {
				skipped++
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, common.AsConnectErr(err)
	}
	return added, skipped, nil
}

func lockDraftSession(tx *gorm.DB, id string) (*model.StocktakeSession, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id required"))
	}
	var sess model.StocktakeSession
	err := common.RowLock(tx).Where("id = ?", id).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("stocktake not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sess.Status != stocktakeStatusDraft {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("stocktake is %s; only DRAFT sessions can be mutated", sess.Status))
	}
	return &sess, nil
}

func (s *StocktakeService) loadLine(ctx context.Context, id string) (*stocktakeifacev1.StocktakeLine, error) {
	type lineRow struct {
		model.StocktakeLine
		ProductID   string `gorm:"column:product_id"`
		ProductName string `gorm:"column:product_name"`
		ProductSku  string `gorm:"column:product_sku"`
		BatchNumber string `gorm:"column:batch_number"`
		ExpiryDate  string `gorm:"column:expiry_date"`
	}
	var r lineRow
	err := s.db.WithContext(ctx).
		Table("stocktake_lines AS l").
		Select(`l.*,
		        b.product_id AS product_id,
		        m.name AS product_name,
		        m.sku AS product_sku,
		        b.batch_number AS batch_number,
		        `+common.DayKeyExpr(s.db, "b.expiry_date")+` AS expiry_date`).
		Joins("JOIN batches AS b ON b.id = l.batch_id").
		Joins("JOIN products AS m ON m.id = b.product_id").
		Where("l.id = ?", id).
		Scan(&r).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return lineRowToProto(r.StocktakeLine, r.ProductID, r.ProductName, r.ProductSku, r.BatchNumber, r.ExpiryDate), nil
}

func (s *StocktakeService) hydrateSession(
	ctx context.Context,
	sess *model.StocktakeSession,
) (*stocktakeifacev1.StocktakeSession, error) {
	type counts struct {
		Total    int32 `gorm:"column:total"`
		Counted  int32 `gorm:"column:counted"`
		Variance int32 `gorm:"column:variance"`
	}
	var c counts
	err := s.db.WithContext(ctx).
		Table("stocktake_lines").
		Select(`COUNT(*) AS total,
		        COUNT(counted_qty) AS counted,
		        COUNT(*) FILTER (WHERE counted_qty IS NOT NULL AND counted_qty <> expected_qty) AS variance`).
		Where("session_id = ?", sess.ID).
		Scan(&c).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := sessionToProto(sess, c.Total, c.Counted, c.Variance)
	// Resolve the warehouse name for display.
	if sess.WarehouseID != nil && *sess.WarehouseID != "" {
		var wh model.Warehouse
		if err := s.db.WithContext(ctx).Select("name").
			Where("id = ?", *sess.WarehouseID).First(&wh).Error; err == nil {
			out.WarehouseName = wh.Name
		}
	}
	return out, nil
}
