package transfer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *TransferService) CreateTransfer(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.CreateTransferRequest],
) (*connect.Response[warehouseifacev1.CreateTransferResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	from := req.Msg.FromWarehouseId
	to := req.Msg.ToWarehouseId
	if from == "" || to == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("from and to warehouse required"))
	}
	if from == to {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source and destination must differ"))
	}
	if len(req.Msg.Lines) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one line required"))
	}

	var transferID string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Both warehouses must exist + be active.
		for _, id := range []string{from, to} {
			var wh model.Warehouse
			if e := tx.Where("id = ? AND active", id).First(&wh).Error; e != nil {
				if errors.Is(e, gorm.ErrRecordNotFound) {
					return connect.NewError(connect.CodeFailedPrecondition, errors.New("warehouse not found or inactive"))
				}
				return connect.NewError(connect.CodeInternal, e)
			}
		}

		now := time.Now()
		no, e := assignTransferNo(tx, now)
		if e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		header := model.StockTransfer{
			TransferNo:      &no,
			FromWarehouseID: from,
			ToWarehouseID:   to,
			Note:            req.Msg.Note,
			CreatedBy:       caller.UserID,
		}
		if e := tx.Create(&header).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		transferID = header.ID

		// Lock the line batches FOR UPDATE (deterministic id order) so concurrent
		// drains of the same source lot serialize and can't both pass the
		// availability check and oversell the source warehouse.
		batchIDs := make([]string, 0, len(req.Msg.Lines))
		for _, line := range req.Msg.Lines {
			batchIDs = append(batchIDs, line.BatchId)
		}
		if e := common.LockBatchesByID(tx, batchIDs); e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}

		for _, line := range req.Msg.Lines {
			if line.Qty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("qty must be > 0"))
			}
			avail, e := common.BatchQtyInWarehouse(ctx, tx, line.BatchId, from)
			if e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			if int64(line.Qty) > avail {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("insufficient stock for batch %s in source warehouse (have %d, need %d)",
						line.BatchId, avail, line.Qty))
			}
			out := model.StockMovement{
				BatchID:     line.BatchId,
				Qty:         -line.Qty,
				Type:        movementTransferOut,
				Reason:      fmt.Sprintf("Transfer %s", no),
				UserID:      caller.UserID,
				WarehouseID: from,
				TransferID:  &transferID,
			}
			in := model.StockMovement{
				BatchID:     line.BatchId,
				Qty:         line.Qty,
				Type:        movementTransferIn,
				Reason:      fmt.Sprintf("Transfer %s", no),
				UserID:      caller.UserID,
				WarehouseID: to,
				TransferID:  &transferID,
			}
			if e := tx.Create(&out).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
			if e := tx.Create(&in).Error; e != nil {
				return connect.NewError(connect.CodeInternal, e)
			}
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	out, err := s.loadTransfer(ctx, transferID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.CreateTransferResponse{Transfer: out}), nil
}
