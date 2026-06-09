package stocktake

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) VoidStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.VoidStocktakeRequest],
) (*connect.Response[stocktakeifacev1.VoidStocktakeResponse], error) {
	var session model.StocktakeSession
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess, ierr := lockDraftSession(tx, req.Msg.SessionId)
		if ierr != nil {
			return ierr
		}
		now := time.Now()
		if ierr := tx.Model(sess).Updates(map[string]any{
			"status":    stocktakeStatusVoided,
			"voided_at": now,
		}).Error; ierr != nil {
			return ierr
		}
		sess.Status = stocktakeStatusVoided
		sess.VoidedAt = &now
		session = *sess
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	out, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.VoidStocktakeResponse{Session: out}), nil
}
