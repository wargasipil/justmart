package stocktake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	stocktakeifacev1 "github.com/justmart/backend/gen/stocktake_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *StocktakeService) CompleteStocktake(
	ctx context.Context,
	req *connect.Request[stocktakeifacev1.CompleteStocktakeRequest],
) (*connect.Response[stocktakeifacev1.CompleteStocktakeResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var movementsWritten int32
	var session model.StocktakeSession
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess, ierr := lockDraftSession(tx, req.Msg.SessionId)
		if ierr != nil {
			return ierr
		}
		session = *sess

		var lines []model.StocktakeLine
		if ierr := tx.Where("session_id = ?", session.ID).Find(&lines).Error; ierr != nil {
			return connect.NewError(connect.CodeInternal, ierr)
		}

		// Lock the lines' batch lots FOR UPDATE (deterministic id order) so the
		// per-line negative guard is reliable against a concurrent sale of the
		// same lot.
		seen := make(map[string]struct{}, len(lines))
		batchIDs := make([]string, 0, len(lines))
		for _, l := range lines {
			if _, ok := seen[l.BatchID]; ok {
				continue
			}
			seen[l.BatchID] = struct{}{}
			batchIDs = append(batchIDs, l.BatchID)
		}
		if ierr := common.LockBatchesByID(tx, batchIDs); ierr != nil {
			return connect.NewError(connect.CodeInternal, ierr)
		}

		// Validate every line before writing any movement.
		for _, l := range lines {
			if l.CountedQty == nil {
				continue
			}
			variance := *l.CountedQty - l.ExpectedQty
			if l.Disposition == dispositionWriteOff {
				if l.WriteOffKind == nil || *l.WriteOffKind == "" {
					return connect.NewError(connect.CodeFailedPrecondition,
						fmt.Errorf("line %s: WRITE_OFF disposition requires a write_off_kind", l.ID))
				}
				if variance > 0 {
					return connect.NewError(connect.CodeFailedPrecondition,
						fmt.Errorf("line %s: positive variance (%d) cannot be a WRITE_OFF", l.ID, variance))
				}
			}
		}

		// Write one movement per counted line with non-zero variance.
		for _, l := range lines {
			if l.CountedQty == nil {
				continue
			}
			variance := *l.CountedQty - l.ExpectedQty
			if variance == 0 {
				continue
			}
			lineID := l.ID
			reasonParts := []string{fmt.Sprintf("Stocktake: %s", session.Name)}
			if l.Disposition == dispositionWriteOff && l.WriteOffKind != nil {
				reasonParts = append(reasonParts, *l.WriteOffKind)
			}
			if l.DispositionNote != "" {
				reasonParts = append(reasonParts, l.DispositionNote)
			}
			mv := model.StockMovement{
				BatchID:         l.BatchID,
				Qty:             variance,
				Type:            l.Disposition,
				Reason:          strings.Join(reasonParts, " — "),
				UserID:          caller.UserID,
				WarehouseID:     common.Deref(session.WarehouseID),
				StocktakeLineID: &lineID,
				WriteOffKind:    l.WriteOffKind,
			}
			if ierr := tx.Create(&mv).Error; ierr != nil {
				return connect.NewError(connect.CodeInternal, ierr)
			}
			// Guard: refuse if the resulting stock in this warehouse goes negative.
			qty, ierr := common.BatchQtyInWarehouse(ctx, tx, l.BatchID, common.Deref(session.WarehouseID))
			if ierr != nil {
				return connect.NewError(connect.CodeInternal, ierr)
			}
			if qty < 0 {
				return connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("line %s: applying variance would drive stock negative", l.ID))
			}
			movementsWritten++
		}

		now := time.Now()
		if ierr := tx.Model(&session).Updates(map[string]any{
			"status":       stocktakeStatusCompleted,
			"completed_at": now,
		}).Error; ierr != nil {
			return connect.NewError(connect.CodeInternal, ierr)
		}
		session.Status = stocktakeStatusCompleted
		session.CompletedAt = &now
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}
	out, err := s.hydrateSession(ctx, &session)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&stocktakeifacev1.CompleteStocktakeResponse{
		Session:          out,
		MovementsWritten: movementsWritten,
	}), nil
}
