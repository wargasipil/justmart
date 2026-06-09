package stocktake

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) StartStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.StartStocktakeRequest],
) (*connect.Response[stocktakeifacev1.StartStocktakeResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}
	// One DRAFT per warehouse — reject only if a draft is already open in THIS
	// warehouse (counting two warehouses concurrently is allowed).
	var existing int64
	if err := s.db.WithContext(ctx).Model(&model.StocktakeSession{}).
		Where("status = ? AND warehouse_id = ?", stocktakeStatusDraft, warehouseID).
		Count(&existing).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if existing > 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("a draft stocktake is already open in this warehouse"))
	}

	session := model.StocktakeSession{
		Name:        strings.TrimSpace(req.Msg.Name),
		Status:      stocktakeStatusDraft,
		CreatedBy:   caller.UserID,
		WarehouseID: &warehouseID,
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.StartStocktakeResponse{Session: out}), nil
}
