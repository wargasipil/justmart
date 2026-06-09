package purchasing

import (
	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func poToProto(po *model.PurchaseOrder) *purchasingifacev1.PurchaseOrder {
	out := &purchasingifacev1.PurchaseOrder{
		Id:           po.ID,
		SupplierId:   po.SupplierID,
		Status:       poStatusFromString(po.Status),
		Note:         po.Note,
		InvoiceNo:    po.InvoiceNo,
		Subtotal:     po.Subtotal,
		CartDiscount: po.CartDiscount,
		PpnEnabled:   po.PpnEnabled,
		PpnRate:      po.PpnRate,
		PpnAmount:    po.PpnAmount,
		OrderedTotal: po.OrderedTotal,
		PaidAmount:   po.PaidAmount,
		Outstanding:  po.OrderedTotal - po.PaidAmount,
		CreatedBy:    po.CreatedBy,
		CreatedAt:    po.CreatedAt.Unix(),
		WarehouseId:  po.WarehouseID,
	}
	if po.PoNo != nil {
		out.PoNo = *po.PoNo
	}
	if po.InvoiceDate != nil {
		out.InvoiceDate = po.InvoiceDate.Format("2006-01-02")
	}
	if po.DueAt != nil {
		out.DueAt = po.DueAt.Format("2006-01-02")
	}
	if po.BranchID != nil {
		out.BranchId = *po.BranchID
	}
	if po.SentAt != nil {
		out.SentAt = po.SentAt.Unix()
	}
	if po.ClosedAt != nil {
		out.ClosedAt = po.ClosedAt.Unix()
	}
	for i := range po.Items {
		out.Items = append(out.Items, poItemToProto(&po.Items[i]))
	}
	return out
}

func poItemToProto(it *model.PurchaseOrderItem) *purchasingifacev1.PurchaseOrderItem {
	factor := it.UnitFactor
	if factor < 1 {
		factor = 1
	}
	out := &purchasingifacev1.PurchaseOrderItem{
		Id:              it.ID,
		PurchaseOrderId: it.PurchaseOrderID,
		ProductId:       it.ProductID,
		OrderedQty:      it.OrderedQty,
		ReceivedQty:     it.ReceivedQty,
		UnitCostPrice:   it.UnitCostPrice,
		Subtotal:        it.Subtotal,
		UnitName:        it.UnitName,
		UnitFactor:      factor,
	}
	if it.ProductUnitID != nil {
		out.ProductUnitId = *it.ProductUnitID
	}
	return out
}

func poStatusToString(s purchasingifacev1.POStatus) string {
	switch s {
	case purchasingifacev1.POStatus_PO_STATUS_DRAFT:
		return poStatusDraft
	case purchasingifacev1.POStatus_PO_STATUS_SENT:
		return poStatusSent
	case purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED:
		return poStatusPartiallyReceived
	case purchasingifacev1.POStatus_PO_STATUS_RECEIVED:
		return poStatusReceived
	case purchasingifacev1.POStatus_PO_STATUS_CLOSED:
		return poStatusClosed
	case purchasingifacev1.POStatus_PO_STATUS_VOIDED:
		return poStatusVoided
	default:
		return ""
	}
}

func poStatusFromString(s string) purchasingifacev1.POStatus {
	switch s {
	case poStatusDraft:
		return purchasingifacev1.POStatus_PO_STATUS_DRAFT
	case poStatusSent:
		return purchasingifacev1.POStatus_PO_STATUS_SENT
	case poStatusPartiallyReceived:
		return purchasingifacev1.POStatus_PO_STATUS_PARTIALLY_RECEIVED
	case poStatusReceived:
		return purchasingifacev1.POStatus_PO_STATUS_RECEIVED
	case poStatusClosed:
		return purchasingifacev1.POStatus_PO_STATUS_CLOSED
	case poStatusVoided:
		return purchasingifacev1.POStatus_PO_STATUS_VOIDED
	default:
		return purchasingifacev1.POStatus_PO_STATUS_UNSPECIFIED
	}
}

func receiptToProto(r *model.PurchaseReceipt) *purchasingifacev1.PurchaseReceipt {
	out := &purchasingifacev1.PurchaseReceipt{
		Id:              r.ID,
		PurchaseOrderId: r.PurchaseOrderID,
		ReceivedAt:      r.ReceivedAt.Format("2006-01-02"),
		ReceivedBy:      r.ReceivedBy,
		Note:            r.Note,
		InvoiceNo:       r.InvoiceNo,
		CreatedAt:       r.CreatedAt.Unix(),
	}
	if r.ReceiptNo != nil {
		out.ReceiptNo = *r.ReceiptNo
	}
	for i := range r.Items {
		out.Items = append(out.Items, receiptItemToProto(&r.Items[i]))
	}
	return out
}

func receiptItemToProto(it *model.PurchaseReceiptItem) *purchasingifacev1.PurchaseReceiptItem {
	factor := it.UnitFactor
	if factor < 1 {
		factor = 1
	}
	out := &purchasingifacev1.PurchaseReceiptItem{
		Id:                  it.ID,
		PurchaseReceiptId:   it.PurchaseReceiptID,
		PurchaseOrderItemId: it.PurchaseOrderItemID,
		ProductId:           it.ProductID,
		Qty:                 it.Qty,
		UnitCostPrice:       it.UnitCostPrice,
		BatchNumber:         it.BatchNumber,
		ExpiryDate:          it.ExpiryDate.Format("2006-01-02"),
		UnitName:            it.UnitName,
		UnitFactor:          factor,
	}
	if it.BatchID != nil {
		out.BatchId = *it.BatchID
	}
	if it.ProductUnitID != nil {
		out.ProductUnitId = *it.ProductUnitID
	}
	return out
}
