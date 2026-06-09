package stocktake

import (
	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func sessionToProto(s *model.StocktakeSession, total, counted, variance int32) *stocktakeifacev1.StocktakeSession {
	out := &stocktakeifacev1.StocktakeSession{
		Id:            s.ID,
		Name:          s.Name,
		Status:        s.Status,
		CreatedBy:     s.CreatedBy,
		CreatedAt:     s.CreatedAt.Unix(),
		LineCount:     total,
		CountedCount:  counted,
		VarianceCount: variance,
	}
	if s.BranchID != nil {
		out.BranchId = *s.BranchID
	}
	if s.WarehouseID != nil {
		out.WarehouseId = *s.WarehouseID
	}
	if s.CompletedAt != nil {
		out.CompletedAt = s.CompletedAt.Unix()
	}
	if s.VoidedAt != nil {
		out.VoidedAt = s.VoidedAt.Unix()
	}
	return out
}

func lineRowToProto(
	l model.StocktakeLine,
	productID, productName, productSku, batchNumber, expiryDate string,
) *stocktakeifacev1.StocktakeLine {
	out := &stocktakeifacev1.StocktakeLine{
		Id:              l.ID,
		SessionId:       l.SessionID,
		BatchId:         l.BatchID,
		ProductId:       productID,
		ProductName:     productName,
		ProductSku:      productSku,
		BatchNumber:     batchNumber,
		ExpiryDate:      expiryDate,
		ExpectedQty:     l.ExpectedQty,
		Disposition:     l.Disposition,
		DispositionNote: l.DispositionNote,
	}
	if l.WriteOffKind != nil {
		out.WriteOffKind = *l.WriteOffKind
	}
	if l.CountedQty != nil {
		out.Counted = true
		out.CountedQty = *l.CountedQty
		out.Variance = *l.CountedQty - l.ExpectedQty
	}
	return out
}
