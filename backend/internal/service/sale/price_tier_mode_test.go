package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
	salesvc "github.com/justmart/backend/internal/service/sale"
)

// These pin test-specification.md › Pos: grosir must behave IDENTICALLY in
// pharmacy and retail mode. Unlike the Rx gate it sits next to in AddItem,
// wholesale pricing is deliberately NOT mode-gated — a toko and an apotek both
// sell in bulk. If someone ever wraps applyTierPrice in an IsPharmacyMode check,
// one of these fails.

func addAndAssertGrosir(t *testing.T, svc *salesvc.SaleService, ctx context.Context, prodID string) {
	t.Helper()
	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 12,
	}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot)
	require.Equal(t, int32(12), it.TierMinQty)
	require.Equal(t, int64(102000), it.LineTotal)
}

func TestPriceTier_RetailMode(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	setRetailMode(t, db)
	prodID, _ := ladder(t, db, "GM1")
	addAndAssertGrosir(t, svc, ctx, prodID)
}

func TestPriceTier_PharmacyMode(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID, _ := ladder(t, db, "GM2")
	addAndAssertGrosir(t, svc, ctx, prodID)
}

// The hard case: pharmacy mode, an Rx-REQUIRED product with a covering
// prescription attached. The Rx gate runs right after applyTierPrice in AddItem,
// so this proves the two features compose rather than fight.
func TestPriceTier_PharmacyMode_RxProductStillGetsGrosir(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)
	setPharmacyMode(t, db)
	prodID := seedRxProduct(t, db, "GM3", "Amoxicillin", 10000)
	var base model.ProductUnit
	require.NoError(t, db.Where("product_id = ? AND is_base", prodID).First(&base).Error)
	seedTier(t, db, prodID, base.ID, 12, 8500)
	seedStock(t, db, prodID, ownerID, 500)

	custID := seedCustomer(t, db, "Pak Budi")
	rxID := seedPrescription(t, db, custID, ownerID, prodID, 50)
	saleID := startDraft(t, svc, ctx)
	_, err := svc.AttachPrescription(ctx, connect.NewRequest(&posifacev1.AttachPrescriptionRequest{
		SaleId: saleID, PrescriptionId: rxID,
	}))
	require.NoError(t, err)

	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: prodID, Qty: 12,
	}))
	require.NoError(t, err)
	it := add.Msg.Sale.Items[0]
	require.Equal(t, int64(8500), it.UnitPriceSnapshot, "Rx coverage doesn't suppress grosir")
	require.Equal(t, int32(12), it.TierMinQty)

	comp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 102000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(102000), comp.Msg.Sale.Total)
}
