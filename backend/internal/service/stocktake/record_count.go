package stocktake

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) RecordCount(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.RecordCountRequest],
) (*connect.Response[stocktakeifacev1.RecordCountResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.CountedQty < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("counted_qty must be >= 0"))
	}
	var line model.StocktakeLine
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if ierr := tx.Where("id = ?", req.Msg.LineId).First(&line).Error; ierr != nil {
			if errors.Is(ierr, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("line not found"))
			}
			return connect.NewError(connect.CodeInternal, ierr)
		}
		if _, ierr := lockDraftSession(tx, line.SessionID); ierr != nil {
			return ierr
		}
		now := time.Now()
		qty := req.Msg.CountedQty
		userID := caller.UserID
		return tx.Model(&line).Updates(map[string]any{
			"counted_qty": qty,
			"counted_at":  now,
			"counted_by":  userID,
		}).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	full, err := s.loadLine(ctx, req.Msg.LineId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.RecordCountResponse{Line: full}), nil
}
