package priceagreement

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *PriceAgreementService) ListPriceAgreements(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ListPriceAgreementsRequest],
) (*connect.Response[inventoryifacev1.ListPriceAgreementsResponse], error) {
	limit, offset := common.NormPage(req.Msg.Limit, req.Msg.Offset)
	supplierID := strings.TrimSpace(req.Msg.SupplierId)
	productID := strings.TrimSpace(req.Msg.ProductId)
	query := strings.TrimSpace(req.Msg.Query)
	today := time.Now().Format(common.DateLayout) // server-local "today" for the validity filter

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if !req.Msg.IncludeInactive {
			q = q.Where("active = ?", true)
		}
		if supplierID != "" {
			q = q.Where("supplier_id = ?", supplierID)
		}
		if productID != "" {
			q = q.Where("product_id = ?", productID)
		}
		if query != "" {
			pattern := "%" + query + "%"
			like := common.LikeOp(q)
			prodSub := s.db.Model(&model.Product{}).Select("id").
				Where("name "+like+" ? OR sku "+like+" ?", pattern, pattern)
			supSub := s.db.Model(&model.Supplier{}).Select("id").
				Where("name "+like+" ? OR code "+like+" ?", pattern, pattern)
			q = q.Where("product_id IN (?) OR supplier_id IN (?)", prodSub, supSub)
		}
		// Validity-window filter, computed against today (server-local). Date
		// columns are normalized to YYYY-MM-DD via DayKeyExpr so the string
		// comparison is engine-agnostic (NULL dates = open-ended).
		switch req.Msg.Validity {
		case inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_CURRENT:
			vf := common.DayKeyExpr(q, "valid_from")
			vu := common.DayKeyExpr(q, "valid_until")
			q = q.Where("(valid_from IS NULL OR "+vf+" <= ?) AND (valid_until IS NULL OR "+vu+" >= ?)", today, today)
		case inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_EXPIRED:
			vu := common.DayKeyExpr(q, "valid_until")
			q = q.Where("valid_until IS NOT NULL AND "+vu+" < ?", today)
		case inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_UPCOMING:
			vf := common.DayKeyExpr(q, "valid_from")
			q = q.Where("valid_from IS NOT NULL AND "+vf+" > ?", today)
		}
		return q
	}

	var total int64
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.PriceAgreement{})).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rows []model.PriceAgreement
	if err := applyFilters(s.db.WithContext(ctx).Model(&model.PriceAgreement{})).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*inventoryifacev1.PriceAgreement, 0, len(rows))
	for i := range rows {
		out = append(out, agreementToProto(&rows[i]))
	}
	return connect.NewResponse(&inventoryifacev1.ListPriceAgreementsResponse{
		Agreements: out,
		Total:      int32(total),
	}), nil
}
