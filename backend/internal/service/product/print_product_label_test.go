package product_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// fakePusher is a ConnectorPusher that records the last Push, standing in for
// the live connector registry.
type fakePusher struct {
	called      bool
	deviceID    string
	printerName string
	payload     []byte
	err         error
}

func (f *fakePusher) Push(deviceID, printerName string, payload []byte) (string, error) {
	f.called = true
	f.deviceID, f.printerName, f.payload = deviceID, printerName, payload
	if f.err != nil {
		return "", f.err
	}
	return "job-1", nil
}

// labelEnv wires a ProductService in connector mode with a fake pusher, so the
// rendered bytes are inspectable without a printer.
func labelEnv(t *testing.T) (*productsvc.ProductService, *gorm.DB, context.Context, *fakePusher) {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	fake := &fakePusher{}
	svc.SetPrinter(cfg.Printer, config.Connector{Mode: "connector"}, fake)
	return svc, gormDB, servicetest.OwnerCtx(context.Background(), ownerID), fake
}

// addUnit attaches a larger sellable unit to a product and returns its id.
func addUnit(t *testing.T, db *gorm.DB, productID, name string, factor, sellPrice int64, active bool) string {
	t.Helper()
	u := model.ProductUnit{
		ProductID: productID,
		Name:      name,
		Factor:    factor,
		IsBase:    false,
		SellPrice: sellPrice,
		Sellable:  true,
		Active:    true,
	}
	require.NoError(t, db.Create(&u).Error)
	if !active {
		// `Active` carries gorm:"default:true", so a false in the struct is the
		// zero value GORM omits and the column default wins — archive explicitly.
		require.NoError(t, db.Model(&model.ProductUnit{}).Where("id = ?", u.ID).
			Update("active", false).Error)
	}
	return u.ID
}

// Happy path: the label reaches the printer carrying the product name, a
// CODE128 symbol of the SKU, and the base unit's price.
func TestPrintProductLabel_PrintsNameBarcodeAndPrice(t *testing.T) {
	t.Parallel()
	svc, _, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-1", "Paracetamol", 2500)

	resp, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.NoError(t, err)
	require.True(t, fake.called)
	require.EqualValues(t, 1, resp.Msg.Copies)
	require.EqualValues(t, len(fake.payload), resp.Msg.BytesSent)

	out := string(fake.payload)
	require.Contains(t, out, "Paracetamol")
	require.Contains(t, out, "Rp 2.500 / tablet")
	// GS k 73 = CODE128, then the length-prefixed {B payload.
	i := bytes.Index(fake.payload, []byte{0x1D, 'k', 73})
	require.GreaterOrEqual(t, i, 0, "no CODE128 command in the payload")
	n := int(fake.payload[i+3])
	require.Equal(t, "{BSKU-LBL-1", string(fake.payload[i+4:i+4+n]))
}

// The unit picker is the whole point of the dialog: a box label must carry the
// box price, not the base price.
func TestPrintProductLabel_UsesChosenUnitPrice(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-2", "Amoxicillin", 1000)
	boxID := addUnit(t, db, id, "box", 100, 90000, true)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id, ProductUnitId: boxID,
	}))
	require.NoError(t, err)
	out := string(fake.payload)
	require.Contains(t, out, "Rp 90.000 / box")
	require.NotContains(t, out, "Rp 1.000")
}

// An empty unit id means the base unit — never "whatever unit sorts first".
func TestPrintProductLabel_EmptyUnitUsesBaseUnit(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-3", "Vitamin C", 700)
	addUnit(t, db, id, "box", 50, 30000, true)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.NoError(t, err)
	require.Contains(t, string(fake.payload), "Rp 700 / tablet")
}

func TestPrintProductLabel_CopiesRepeatTheLabel(t *testing.T) {
	t.Parallel()
	svc, _, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-4", "Antasida", 1500)

	resp, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id, Copies: 4,
	}))
	require.NoError(t, err)
	require.EqualValues(t, 4, resp.Msg.Copies)
	require.Equal(t, 4, bytes.Count(fake.payload, []byte{0x1D, 'k', 73}))
}

// An over-large request prints the cap rather than failing, and says so.
func TestPrintProductLabel_ClampsCopiesAndEchoesActual(t *testing.T) {
	t.Parallel()
	svc, _, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-5", "Ibuprofen", 3000)

	resp, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id, Copies: 10_000,
	}))
	require.NoError(t, err)
	require.EqualValues(t, 100, resp.Msg.Copies)
	require.Equal(t, 100, bytes.Count(fake.payload, []byte{0x1D, 'k', 73}))
}

func TestPrintProductLabel_RejectsNegativeCopies(t *testing.T) {
	t.Parallel()
	svc, _, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-6", "Cetirizine", 1200)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id, Copies: -3,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.False(t, fake.called, "nothing should reach the printer")
}

// A unit id belonging to another product must not silently fall back to the
// base price — that would mislabel the shelf.
func TestPrintProductLabel_RejectsForeignUnitID(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	a := seedProduct(t, svc, ctx, "SKU-LBL-7", "Product A", 1000)
	b := seedProduct(t, svc, ctx, "SKU-LBL-8", "Product B", 2000)
	otherUnit := addUnit(t, db, b, "box", 10, 18000, true)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: a, ProductUnitId: otherUnit,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.False(t, fake.called)
}

func TestPrintProductLabel_RejectsArchivedUnit(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-9", "Loratadine", 900)
	deadUnit := addUnit(t, db, id, "box", 20, 16000, false)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id, ProductUnitId: deadUnit,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "product.unit_archived", ce.Message())
	require.False(t, fake.called)
}

// A SKU the symbology cannot carry must fail loudly: encoding it anyway yields
// a label that scans back as a different string at the till.
func TestPrintProductLabel_RejectsUnprintableSKU(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-10", "Obat Bebas", 800)
	require.NoError(t, db.Model(&model.Product{}).Where("id = ?", id).
		Update("sku", "OBAT-Ké").Error)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "product.sku_not_printable", ce.Message())
	require.False(t, fake.called)
}

func TestPrintProductLabel_UnknownProduct(t *testing.T) {
	t.Parallel()
	svc, _, ctx, fake := labelEnv(t)
	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: "11111111-1111-1111-1111-111111111111",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.False(t, fake.called)
}

// An explicit target goes straight through; this is the same precedence
// PrintReceipt uses, since both share common.PrintDispatcher.
func TestPrintProductLabel_ExplicitConnectorTarget(t *testing.T) {
	t.Parallel()
	svc, _, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-11", "Betadine", 15000)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id, ConnectorDeviceId: "dev-7", PrinterName: "LABEL-58",
	}))
	require.NoError(t, err)
	require.Equal(t, "dev-7", fake.deviceID)
	require.Equal(t, "LABEL-58", fake.printerName)
}

// With no target on the request, the saved Settings ▸ Printing default is used.
func TestPrintProductLabel_FallsBackToSavedTarget(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "SKU-LBL-12", "Hansaplast", 5000)
	require.NoError(t, common.SetPrintTarget(ctx, db, "saved-dev", "saved-printer"))

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.NoError(t, err)
	require.Equal(t, "saved-dev", fake.deviceID)
	require.Equal(t, "saved-printer", fake.printerName)
}

// Printing is off by default (tcp mode + printer.enabled false): the RPC must
// say so rather than dialing nothing.
func TestPrintProductLabel_NotConfigured(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	svc := productsvc.NewProductService(gormDB)
	cfg.Printer.Enabled = false
	svc.SetPrinter(cfg.Printer, config.Connector{Mode: "tcp"}, nil)
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)
	id := seedProduct(t, svc, ctx, "SKU-LBL-13", "Minyak Kayu Putih", 12000)

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// A SKU too long for the configured paper must be refused, not truncated by the
// printer into a label that looks right and scans as nothing. 16 chars needs 422
// dots; 58mm paper (width 32) prints 384.
func TestPrintProductLabel_RefusesSkuTooWideForPaper(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "RS-1785524168486", "Restock Med", 700)
	require.NoError(t, common.SetReceiptWidth(ctx, db, 32))

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "product.sku_too_wide_for_paper", ce.Message())
	require.False(t, fake.called, "nothing should reach the printer")
}

// The same SKU prints on 80mm paper (576 dots).
func TestPrintProductLabel_LongSkuFitsWiderPaper(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "RS-1785524168487", "Restock Med", 700)
	require.NoError(t, common.SetReceiptWidth(ctx, db, 48))

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.NoError(t, err)
	require.True(t, fake.called)
}

// Bar width is derived from the SKU length, not fixed: a short code gets wider,
// more scannable bars on the same paper.
func TestPrintProductLabel_WidensBarsForShortSku(t *testing.T) {
	t.Parallel()
	svc, db, ctx, fake := labelEnv(t)
	id := seedProduct(t, svc, ctx, "AB123", "Short SKU", 500)
	require.NoError(t, common.SetReceiptWidth(ctx, db, 32))

	_, err := svc.PrintProductLabel(ctx, connect.NewRequest(&inventoryifacev1.PrintProductLabelRequest{
		ProductId: id,
	}))
	require.NoError(t, err)
	require.Contains(t, string(fake.payload), string([]byte{0x1D, 'w', 3}), "GS w should be 3")
}
