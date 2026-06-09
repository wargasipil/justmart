package stock

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StockService) RecordMovement(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.RecordMovementRequest],
) (*connect.Response[inventoryifacev1.RecordMovementResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	if req.Msg.BatchId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("batch_id required"))
	}
	if req.Msg.Qty == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("qty must not be zero"))
	}

	// Restrict allowed types for this RPC. PURCHASE comes via CreateBatch;
	// SALE will come from POS in a later phase.
	var typeStr string
	switch req.Msg.Type {
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT:
		typeStr = "ADJUSTMENT"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF:
		typeStr = "WRITE_OFF"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("type must be ADJUSTMENT or WRITE_OFF for RecordMovement"))
	}

	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	mv := model.StockMovement{
		BatchID:     req.Msg.BatchId,
		Qty:         req.Msg.Qty,
		Type:        typeStr,
		Reason:      req.Msg.Reason,
		UserID:      caller.UserID,
		WarehouseID: warehouseID,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the batch lot FOR UPDATE first so the post-insert negative guard is
		// reliable against a concurrent sale/transfer of the same lot.
		if err := common.LockBatchesByID(tx, []string{mv.BatchID}); err != nil {
			return err
		}
		if err := tx.Create(&mv).Error; err != nil {
			return fmt.Errorf("create movement: %w", err)
		}
		// Guard: refuse if this movement would drive the batch's stock in this
		// warehouse negative.
		qty, err := common.BatchQtyInWarehouse(ctx, tx, mv.BatchID, warehouseID)
		if err != nil {
			return err
		}
		if qty < 0 {
			return fmt.Errorf("movement would drive stock negative (current=%d)", qty)
		}
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	return connect.NewResponse(&inventoryifacev1.RecordMovementResponse{Movement: movementToProto(&mv)}), nil
}
