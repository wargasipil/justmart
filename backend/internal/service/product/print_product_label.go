package product

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/printer"
	"github.com/justmart/backend/internal/service/common"
)

// PrintProductLabel renders a shelf label — product name, a CODE128 barcode of
// the SKU, and the chosen unit's sell price — and sends it to the shop's
// thermal printer through the shared dispatcher (connector / usb / raw TCP).
//
// The SKU *is* the barcode in this system: POS scans by exact-SKU match, so a
// label printed here is guaranteed to resolve at the till.
func (s *ProductService) PrintProductLabel(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.PrintProductLabelRequest],
) (*connect.Response[inventoryifacev1.PrintProductLabelResponse], error) {
	if s.print == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("printing is not wired on this server"))
	}
	if err := s.print.EnsureConfigured(); err != nil {
		return nil, err
	}

	copies, err := normLabelCopies(req.Msg.Copies)
	if err != nil {
		return nil, err
	}

	prod, err := s.load(ctx, req.Msg.ProductId)
	if err != nil {
		return nil, err
	}
	// A SKU carrying a non-printable or non-ASCII byte would be encoded by the
	// printer into a symbol that scans back as something else. Refuse rather
	// than emit a label that silently rings up the wrong product.
	if !printer.Code128Encodable(prod.SKU) {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "product.sku_not_printable")
	}

	unitName, price, err := s.labelUnit(ctx, prod, req.Msg.ProductUnitId)
	if err != nil {
		return nil, err
	}

	width := s.print.PrinterWidth()
	if w, werr := common.GetReceiptWidth(ctx, s.db); werr == nil {
		width = int(w)
	}

	// Bars are sized to the paper, not fixed: a long SKU at a fixed module width
	// overflows the print head, and the printer's response to that is to truncate
	// the symbol — a label that looks right and scans as nothing. Refuse instead.
	barWidth, fits := printer.FitModuleWidth(len(prod.SKU), printer.PaperDots(width))
	if !fits {
		return nil, common.TokenError(connect.CodeFailedPrecondition, "product.sku_too_wide_for_paper")
	}

	payload := printer.RenderLabel(
		printer.Label{
			Name:     prod.Name,
			SKU:      prod.SKU,
			UnitName: unitName,
			Price:    price,
		},
		printer.LabelSettings{Width: width, Copies: copies, BarcodeWidth: barWidth},
	)

	if err := s.print.Dispatch(
		ctx, s.db,
		req.Msg.ConnectorDeviceId, req.Msg.PrinterName,
		payload, "label-"+prod.ID,
	); err != nil {
		return nil, err
	}

	return connect.NewResponse(&inventoryifacev1.PrintProductLabelResponse{
		BytesSent: int32(len(payload)),
		Copies:    int32(copies),
	}), nil
}

// normLabelCopies defaults 0 to one label and clamps the ceiling rather than
// failing, so an over-large request still prints something useful; the response
// echoes what was actually printed. A negative count is a caller bug, not a
// default, so it is rejected.
func normLabelCopies(n int32) (int, error) {
	if n < 0 {
		return 0, connect.NewError(connect.CodeInvalidArgument, errors.New("copies must not be negative"))
	}
	if n == 0 {
		return 1, nil
	}
	if int(n) > printer.MaxLabelCopies {
		return printer.MaxLabelCopies, nil
	}
	return int(n), nil
}

// labelUnit resolves which unit's name + price the label prints. An empty
// unitID means the base unit; the product's own unit/unit_price is the fallback
// for a catalog row that predates the units table.
func (s *ProductService) labelUnit(
	ctx context.Context,
	prod *model.Product,
	unitID string,
) (string, int64, error) {
	q := s.db.WithContext(ctx).Where("product_id = ?", prod.ID)
	if unitID == "" {
		q = q.Where("is_base")
	} else {
		q = q.Where("id = ?", unitID)
	}

	var unit model.ProductUnit
	err := q.First(&unit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if unitID != "" {
			// An explicit id that isn't this product's unit — never silently fall
			// back to the base price, that would mislabel the shelf.
			return "", 0, connect.NewError(connect.CodeNotFound,
				errors.New("unit not found for this product"))
		}
		return prod.Unit, prod.UnitPrice, nil
	}
	if err != nil {
		return "", 0, connect.NewError(connect.CodeInternal, err)
	}
	if !unit.Active {
		return "", 0, common.TokenError(connect.CodeFailedPrecondition, "product.unit_archived")
	}
	return unit.Name, unit.SellPrice, nil
}
