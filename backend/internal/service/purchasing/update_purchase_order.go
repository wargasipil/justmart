package purchasing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (p *PurchaseOrders) UpdatePurchaseOrder(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.UpdatePurchaseOrderRequest],
) (*connect.Response[purchasingifacev1.UpdatePurchaseOrderResponse], error) {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po, err := p.lockByID(tx, req.Msg.Id)
		if err != nil {
			return err
		}
		if po.Status != poStatusDraft {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("only DRAFT POs are editable; this one is %s", po.Status))
		}
		updates := map[string]any{
			"note":        strings.TrimSpace(req.Msg.Note),
			"invoice_no":  strings.TrimSpace(req.Msg.InvoiceNo),
			"ppn_enabled": req.Msg.PpnEnabled,
		}
		if e, err := parseDateMaybe(req.Msg.InvoiceDate); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else {
			updates["invoice_date"] = e
		}
		if e, err := parseDateMaybe(req.Msg.DueAt); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		} else {
			updates["due_at"] = e
		}
		if err := tx.Model(po).Updates(updates).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Recompute totals: either with the new items (if provided) or with the
		// existing items reloaded from DB. Either way the PPN/discount inputs
		// from the request drive the math.
		var items []model.PurchaseOrderItem
		if len(req.Msg.Items) > 0 {
			if err := tx.Where("purchase_order_id = ?", po.ID).Delete(&model.PurchaseOrderItem{}).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			for _, in := range req.Msg.Items {
				if in.OrderedQty <= 0 {
					return connect.NewError(connect.CodeInvalidArgument, errors.New("ordered_qty must be > 0"))
				}
				unit, err := resolvePurchaseUnit(tx, in.ProductId, in.ProductUnitId)
				if err != nil {
					return err
				}
				baseQty := in.OrderedQty * int32(unit.Factor) // ordered_qty stored in BASE units
				it := model.PurchaseOrderItem{
					PurchaseOrderID: po.ID,
					ProductID:       in.ProductId,
					OrderedQty:      baseQty,
					UnitCostPrice:   in.UnitCostPrice, // per base unit
					Subtotal:        int64(baseQty) * in.UnitCostPrice,
					ProductUnitID:   &unit.ID,
					UnitName:        unit.Name,
					UnitFactor:      unit.Factor,
				}
				items = append(items, it)
			}
			if err := tx.Create(&items).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		} else {
			if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		subtotal, discount, rate, ppn, total := computePOTotals(items, req.Msg.CartDiscount, req.Msg.PpnEnabled, req.Msg.PpnRate)
		if err := tx.Model(po).Updates(map[string]any{
			"subtotal":      subtotal,
			"cart_discount": discount,
			"ppn_rate":      rate,
			"ppn_amount":    ppn,
			"ordered_total": total,
		}).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := p.loadFull(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&purchasingifacev1.UpdatePurchaseOrderResponse{Order: poToProto(full)}), nil
}
