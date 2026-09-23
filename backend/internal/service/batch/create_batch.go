package batch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *BatchService) CreateBatch(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateBatchRequest],
) (*connect.Response[inventoryifacev1.CreateBatchResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	if req.Msg.ProductId == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.product_required")
	}
	expiry, err := time.Parse(common.DateLayout, req.Msg.ExpiryDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expiry_date must be YYYY-MM-DD: %w", err))
	}
	received := time.Now()
	if req.Msg.ReceivedAt != "" {
		received, err = time.Parse(common.DateLayout, req.Msg.ReceivedAt)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("received_at must be YYYY-MM-DD: %w", err))
		}
	}
	if req.Msg.InitialQuantity < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.qty_negative")
	}
	if req.Msg.CostPrice < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "batch.cost_negative")
	}

	batch := model.Batch{
		ProductID:   req.Msg.ProductId,
		BatchNumber: strings.TrimSpace(req.Msg.BatchNumber),
		ExpiryDate:  expiry,
		CostPrice:   req.Msg.CostPrice,
		ReceivedAt:  received,
	}
	if req.Msg.SupplierId != "" {
		sid := req.Msg.SupplierId
		batch.SupplierID = &sid
	}
	// Who MADE the lot. The manual path is the one place a person is holding
	// the box and can read the factory off it, so refusing to record it would
	// mint a lot that answers no recall while the answer was in front of them.
	//
	// Existence is checked but NOT activity: unlike a product's approved-source
	// list (which refuses an archived pabrik, since you cannot start sourcing
	// from a retired factory), a lot records what happened -- and hand-entered
	// stock may genuinely come from one. Same posture as CreateReceipt.
	if mid := strings.TrimSpace(req.Msg.ManufacturerId); mid != "" {
		var maker model.Manufacturer
		if e := s.db.WithContext(ctx).Where("id = ?", mid).First(&maker).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("manufacturer %s not found", mid))
			}
			return nil, connect.NewError(connect.CodeInternal, e)
		}
		batch.ManufacturerID = &mid
	}

	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return fmt.Errorf("create batch: %w", err)
		}
		if req.Msg.InitialQuantity > 0 {
			mv := model.StockMovement{
				BatchID:     batch.ID,
				Qty:         int32(req.Msg.InitialQuantity),
				Type:        "PURCHASE",
				Reason:      "initial stock",
				UserID:      caller.UserID,
				WarehouseID: warehouseID,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return fmt.Errorf("create initial movement: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	qty := int64(req.Msg.InitialQuantity)
	return connect.NewResponse(&inventoryifacev1.CreateBatchResponse{Batch: batchToProto(&batch, qty)}), nil
}
