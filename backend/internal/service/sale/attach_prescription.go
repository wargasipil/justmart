package sale

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	prescriptionsvc "github.com/justmart/backend/internal/service/prescription"
)

func (s *SaleService) AttachPrescription(
	ctx context.Context,
	req *connect.Request[posifacev1.AttachPrescriptionRequest],
) (*connect.Response[posifacev1.AttachPrescriptionResponse], error) {
	if req.Msg.PrescriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prescription_id required"))
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}

		var rx model.Prescription
		if err := tx.Preload("Items").Where("id = ?", req.Msg.PrescriptionId).First(&rx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("prescription not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if prescriptionsvc.ComputeStatus(&rx, time.Now()) != prescriptionsvc.StatusActive {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("prescription is not active"))
		}

		// Customer consistency: the sale's customer (if any) must be the Rx
		// patient; an unset sale customer is auto-filled from the Rx.
		// Snapshot the resep's biaya jasa (service fee) onto the sale — survives
		// later resep edits, same spirit as unit_price_snapshot; editable at POS.
		updates := map[string]any{"prescription_id": rx.ID, "biaya_jasa": rx.BiayaJasa}
		if sale.CustomerID != nil && *sale.CustomerID != "" {
			if *sale.CustomerID != rx.CustomerID {
				return connect.NewError(connect.CodeFailedPrecondition,
					errors.New("sale customer differs from the prescription patient"))
			}
		} else {
			updates["customer_id"] = rx.CustomerID
		}
		if err := tx.Model(sale).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		// Recompute so the persisted total reflects the fee immediately (the
		// CompleteSale cash guard reads sale.Total).
		return recomputeSaleTotals(tx, sale.ID)
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.AttachPrescriptionResponse{Sale: saleToProto(sale)}), nil
}
