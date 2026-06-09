package stocktake

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) SetLineDisposition(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.SetLineDispositionRequest],
) (*connect.Response[stocktakeifacev1.SetLineDispositionResponse], error) {
	disposition := strings.ToUpper(strings.TrimSpace(req.Msg.Disposition))
	if disposition != dispositionAdjustment && disposition != dispositionWriteOff {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("disposition must be ADJUSTMENT or WRITE_OFF"))
	}
	kind := strings.ToUpper(strings.TrimSpace(req.Msg.WriteOffKind))
	if disposition == dispositionWriteOff {
		if kind == "" || !validWriteOffKinds[kind] {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("write_off_kind must be EXPIRED|DAMAGED|LOST|THEFT|OTHER when disposition is WRITE_OFF"))
		}
	} else {
		kind = "" // strip kind for ADJUSTMENT lines
	}

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
		updates := map[string]any{
			"disposition":      disposition,
			"disposition_note": strings.TrimSpace(req.Msg.DispositionNote),
		}
		if kind == "" {
			updates["write_off_kind"] = nil
		} else {
			updates["write_off_kind"] = kind
		}
		return tx.Model(&line).Updates(updates).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	full, err := s.loadLine(ctx, req.Msg.LineId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.SetLineDispositionResponse{Line: full}), nil
}
