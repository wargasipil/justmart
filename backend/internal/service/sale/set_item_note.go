package sale

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// maxKitchenNoteLen bounds the cook-facing note. It prints on a 32-column
// thermal ticket, so an unbounded string just produces an unreadable wall.
const maxKitchenNoteLen = 120

// SetItemNote sets the cook-facing note on a cart line ("no ice", "extra
// pedas"). "" clears it.
//
// A note NEVER affects any amount. Priced modifiers ("+ extra cheese Rp 5.000")
// are a different feature — they touch line totals, discounts and tier pricing —
// and dressing a note up as one is how the money quietly goes wrong. If a
// modifier should cost something, it belongs in the cart as its own line (a
// SERVICE product does exactly that).
func (s *SaleService) SetItemNote(
	ctx context.Context,
	req *connect.Request[posifacev1.SetItemNoteRequest],
) (*connect.Response[posifacev1.SetItemNoteResponse], error) {
	note := strings.TrimSpace(req.Msg.Note)
	if len([]rune(note)) > maxKitchenNoteLen {
		return nil, common.TokenError(connect.CodeInvalidArgument, "sale.note_too_long")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sale, err := s.draftForUpdate(tx, req.Msg.SaleId)
		if err != nil {
			return err
		}
		var item model.SaleItem
		if err := tx.Where("id = ? AND sale_id = ?", req.Msg.ItemId, sale.ID).
			First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return connect.NewError(connect.CodeNotFound, errors.New("cart line not found"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		// Editing the note of an ALREADY-FIRED line is allowed but does not
		// reprint: the kitchen has the old ticket in hand, so the change has to
		// be spoken. Re-firing on a note edit would print a duplicate of a dish
		// already being cooked.
		return tx.Model(&model.SaleItem{}).
			Where("id = ?", item.ID).
			Update("kitchen_note", note).Error
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	sale, err := s.loadFull(ctx, req.Msg.SaleId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&posifacev1.SetItemNoteResponse{Sale: saleToProto(sale)}), nil
}
