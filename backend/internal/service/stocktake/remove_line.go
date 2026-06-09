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

func (s *StocktakeService) RemoveLine(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.RemoveLineRequest],
) (*connect.Response[stocktakeifacev1.RemoveLineResponse], error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var line model.StocktakeLine
		if ierr := tx.Where("id = ?", req.Msg.LineId).First(&line).Error; ierr != nil {
			if errors.Is(ierr, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("line not found"))
			}
			return connect.NewError(connect.CodeInternal, ierr)
		}
		if _, ierr := lockDraftSession(tx, line.SessionID); ierr != nil {
			return ierr
		}
		return tx.Delete(&line).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	return connect.NewResponse(&stocktakeifacev1.RemoveLineResponse{}), nil
}
