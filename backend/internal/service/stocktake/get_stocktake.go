package stocktake

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) GetStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.GetStocktakeRequest],
) (*connect.Response[stocktakeifacev1.GetStocktakeResponse], error) {
	var session model.StocktakeSession
	if err := s.db.WithContext(ctx).Where("id = ?", req.Msg.Id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("stocktake not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sessProto, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	// Load lines + denormalize batch/product context for display.
	type lineRow struct {
		model.StocktakeLine
		ProductID   string `gorm:"column:product_id"`
		ProductName string `gorm:"column:product_name"`
		ProductSku  string `gorm:"column:product_sku"`
		BatchNumber string `gorm:"column:batch_number"`
		ExpiryDate  string `gorm:"column:expiry_date"`
	}
	var rows []lineRow
	err = s.db.WithContext(ctx).
		Table("stocktake_lines AS l").
		Select(`l.*,
		        b.product_id AS product_id,
		        m.name AS product_name,
		        m.sku AS product_sku,
		        b.batch_number AS batch_number,
		        `+common.DayKeyExpr(s.db, "b.expiry_date")+` AS expiry_date`).
		Joins("JOIN batches AS b ON b.id = l.batch_id").
		Joins("JOIN products AS m ON m.id = b.product_id").
		Where("l.session_id = ?", session.ID).
		Order("l.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	lines := make([]*stocktakeifacev1.StocktakeLine, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, lineRowToProto(r.StocktakeLine, r.ProductID, r.ProductName, r.ProductSku, r.BatchNumber, r.ExpiryDate))
	}
	return connect.NewResponse(&stocktakeifacev1.GetStocktakeResponse{
		Session: sessProto,
		Lines:   lines,
	}), nil
}
