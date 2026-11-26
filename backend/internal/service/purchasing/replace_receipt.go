package purchasing

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// ReplaceReceipt corrects a delivery that was entered with a mistake ("ubah
// penerimaan": wrong qty, batch number, expiry, faktur no). Before it, the shop
// had to Batalkan and then type the whole delivery again in Terima barang.
//
// It is exactly that pair, as ONE transaction: cancelReceiptTx on the old
// receipt -- every CancelReceipt precondition and refusal token unchanged, so
// the UI offers it wherever it offers Batalkan and reuses cancel_blocked_reason
// -- then createReceiptTx for the corrected lines against the SAME purchase
// order, then old.replaced_by_id = new.id. Any refusal (a consumed lot, a
// closed order, a corrected qty above what is still outstanding, a bad expiry)
// rolls everything back and the old delivery stays intact.
//
// The order matters: the cancel runs first so the outstanding qty the new lines
// are checked against already has the old delivery given back, and so the old
// lot is gone before a lot with the same batch number is created.
//
// The corrected delivery gets a NEW RCV number; the old one stays visible,
// voided with the reason, pointing at its replacement. Rewriting the old
// receipt in place would leave nothing to audit against.
func (p *PurchaseReceipts) ReplaceReceipt(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.ReplaceReceiptRequest],
) (*connect.Response[purchasingifacev1.ReplaceReceiptResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	receiptID := strings.TrimSpace(req.Msg.Id)
	if receiptID == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.receipt_required")
	}
	reason := strings.TrimSpace(req.Msg.Reason)
	if reason == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "purchasing.reason_required")
	}
	receivedAt, err := checkReceiptInput(req.Msg.Lines, req.Msg.ReceivedAt)
	if err != nil {
		return nil, err
	}

	var created *model.PurchaseReceipt
	err = p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old, e := cancelReceiptTx(tx, caller, receiptID, reason)
		if e != nil {
			return e
		}
		created, e = createReceiptTx(ctx, tx, caller, &purchasingifacev1.CreateReceiptRequest{
			PurchaseOrderId: old.PurchaseOrderID,
			ReceivedAt:      req.Msg.ReceivedAt,
			Note:            req.Msg.Note,
			Lines:           req.Msg.Lines,
			InvoiceNo:       req.Msg.InvoiceNo,
		}, receivedAt)
		if e != nil {
			return e
		}
		if e := tx.Model(&model.PurchaseReceipt{}).Where("id = ?", old.ID).
			Update("replaced_by_id", created.ID).Error; e != nil {
			return connect.NewError(connect.CodeInternal, e)
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	fullNew, err := p.loadFull(ctx, created.ID)
	if err != nil {
		return nil, err
	}
	fullOld, err := p.loadFull(ctx, receiptID)
	if err != nil {
		return nil, err
	}
	newOut, oldOut := receiptToProto(fullNew), receiptToProto(fullOld)
	if err := attachExpirySources(ctx, p.db, newOut); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := attachReplacedByNo(ctx, p.db, oldOut); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&purchasingifacev1.ReplaceReceiptResponse{
		Receipt:  newOut,
		Replaced: oldOut,
	}), nil
}
