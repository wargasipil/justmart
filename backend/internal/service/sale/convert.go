package sale

import (
	"errors"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func saleToProto(s *model.Sale) *posifacev1.Sale {
	out := &posifacev1.Sale{
		Id:            s.ID,
		CashierUserId: s.CashierUserID,
		Subtotal:      s.Subtotal,
		CartDiscount:  s.CartDiscount,
		Total:         s.Total,
		PaidAmount:    s.PaidAmount,
		Status:        saleStatusToProto(s.Status),
		CreatedAt:     s.CreatedAt.Unix(),
		PaymentSource: paymentSourceFromString(common.Deref(s.PaymentSource)),
	}
	if s.SaleNo != nil {
		out.SaleNo = *s.SaleNo
	}
	if s.CustomerID != nil {
		out.CustomerId = *s.CustomerID
	}
	if s.BranchID != nil {
		out.BranchId = *s.BranchID
	}
	if s.WarehouseID != nil {
		out.WarehouseId = *s.WarehouseID
	}
	if s.PrescriptionID != nil {
		out.PrescriptionId = *s.PrescriptionID
	}
	if s.CompletedAt != nil {
		out.CompletedAt = s.CompletedAt.Unix()
	}
	for i := range s.Items {
		out.Items = append(out.Items, saleItemToProto(&s.Items[i]))
	}
	return out
}

func saleItemToProto(i *model.SaleItem) *posifacev1.SaleItem {
	out := &posifacev1.SaleItem{
		Id:                i.ID,
		SaleId:            i.SaleID,
		ProductId:         i.ProductID,
		Qty:               i.Qty,
		UnitPriceSnapshot: i.UnitPriceSnapshot,
		LineDiscount:      i.LineDiscount,
		LineTotal:         i.LineTotal,
		UnitName:          i.UnitName,
		UnitFactor:        i.UnitFactor,
		BaseQty:           i.BaseQty,
	}
	if i.BatchID != nil {
		out.BatchId = *i.BatchID
	}
	if i.ProductUnitID != nil {
		out.ProductUnitId = *i.ProductUnitID
	}
	return out
}

func saleStatusToString(s posifacev1.SaleStatus) string {
	switch s {
	case posifacev1.SaleStatus_SALE_STATUS_DRAFT:
		return saleStatusDraft
	case posifacev1.SaleStatus_SALE_STATUS_COMPLETED:
		return saleStatusCompleted
	case posifacev1.SaleStatus_SALE_STATUS_VOIDED:
		return saleStatusVoided
	default:
		return ""
	}
}

func saleStatusToProto(s string) posifacev1.SaleStatus {
	switch s {
	case saleStatusDraft:
		return posifacev1.SaleStatus_SALE_STATUS_DRAFT
	case saleStatusCompleted:
		return posifacev1.SaleStatus_SALE_STATUS_COMPLETED
	case saleStatusVoided:
		return posifacev1.SaleStatus_SALE_STATUS_VOIDED
	default:
		return posifacev1.SaleStatus_SALE_STATUS_UNSPECIFIED
	}
}

func paymentSourceToString(p posifacev1.PaymentSource) (string, error) {
	switch p {
	case posifacev1.PaymentSource_PAYMENT_SOURCE_CASH:
		return paymentCash, nil
	case posifacev1.PaymentSource_PAYMENT_SOURCE_NON_CASH:
		return paymentNonCash, nil
	default:
		return "", errors.New("payment_source required")
	}
}

func paymentSourceFromString(s string) posifacev1.PaymentSource {
	switch s {
	case paymentCash:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_CASH
	case paymentNonCash:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_NON_CASH
	default:
		return posifacev1.PaymentSource_PAYMENT_SOURCE_UNSPECIFIED
	}
}
