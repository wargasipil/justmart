package purchasing

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseOrders) CreatePurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.CreatePurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.CreatePurchaseOrderResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.SupplierId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("supplier_id required"))
	}
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item required"))
	}

	warehouseID, err := common.ResolveWarehouse(ctx, p.db, caller)
	if err != nil {
		return nil, err
	}

	var po model.PurchaseOrder
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po = model.PurchaseOrder{
			SupplierID:  req.Msg.SupplierId,
			Status:      poStatusDraft,
			Note:        strings.TrimSpace(req.Msg.Note),
			InvoiceNo:   strings.TrimSpace(req.Msg.InvoiceNo),
			CreatedBy:   caller.UserID,
			WarehouseID: warehouseID,
			PpnEnabled:  req.Msg.PpnEnabled,
		}
		if e, err := parseDateMaybe(req.Msg.InvoiceDate); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else if e != nil {
			po.InvoiceDate = e
		}
		if e, err := parseDateMaybe(req.Msg.DueAt); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else if e != nil {
			po.DueAt = e
		}

		var items []model.PurchaseOrderItem
		for _, in := range req.Msg.Items {
			if in.OrderedQty <= 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("ordered_qty must be > 0"))
			}
			if in.UnitCostPrice < 0 {
				return connect.NewError(connect.CodeInvalidArgument, errors.New("unit_cost_price must be >= 0"))
			}
			unit, err := resolvePurchaseUnit(tx, in.ProductId, in.ProductUnitId)
			if err != nil {
				return err
			}
			baseQty := in.OrderedQty * int32(unit.Factor) // ordered_qty stored in BASE units
			gross := int64(baseQty) * in.UnitCostPrice
			net, discType, err := lineNetSubtotal(gross, in.DiscountType, in.DiscountValue)
			if err != nil {
				return err
			}
			it := model.PurchaseOrderItem{
				ProductID:     in.ProductId,
				OrderedQty:    baseQty,
				UnitCostPrice: in.UnitCostPrice, // GROSS per base unit
				Subtotal:      net,              // NET (after per-line discount)
				DiscountType:  discType,
				DiscountValue: in.DiscountValue,
				ProductUnitID: &unit.ID,
				UnitName:      unit.Name,
				UnitFactor:    unit.Factor,
			}
			items = append(items, it)
		}
		subtotal, discount, rate, ppn, total := computePOTotals(items, req.Msg.CartDiscount, req.Msg.PpnEnabled, req.Msg.PpnRate)
		po.Subtotal = subtotal
		po.CartDiscount = discount
		po.PpnRate = rate
		po.PpnAmount = ppn
		po.OrderedTotal = total

		poNo, err := assignPONo(tx, time.Now())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		po.PoNo = &poNo

		if err := tx.Create(&po).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		for i := range items {
			items[i].PurchaseOrderID = po.ID
		}
		if err := tx.Create(&items).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := p.loadFull(ctx, po.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.CreatePurchaseOrderResponse{Order: poToProto(full)}), nil
}
