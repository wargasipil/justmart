package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// Selling a COMPOSITE consumes its recipe's COMPONENTS, scaled by the quantity
// sold — the menu item itself has no batches and none are invented for it.
func TestCompleteSale_CompositeConsumesIngredients(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)

	rice := seedProduct(t, db, "ing-rice", "Beras", 0)
	egg := seedProduct(t, db, "ing-egg", "Telur", 0)
	seedStock(t, db, rice, ownerID, 1000) // grams
	seedStock(t, db, egg, ownerID, 10)    // pieces

	nasgor := seedKindProduct(t, db, "menu-nasgor", "Nasi Goreng", 25000, compositeKind)
	seedRecipe(t, db, nasgor, rice, 200) // 200 g per portion
	seedRecipe(t, db, nasgor, egg, 1)    // 1 egg per portion

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 3,
	}))
	require.NoError(t, err)

	resp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId:        saleID,
		PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH,
		PaidAmount:    75000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, resp.Msg.Sale.Status)
	require.Equal(t, int64(75000), resp.Msg.Sale.Total)

	// 3 portions -> 600 g rice and 3 eggs deducted.
	require.Equal(t, int64(400), stockOnHand(t, db, rice))
	require.Equal(t, int64(7), stockOnHand(t, db, egg))
}

// Every exploded movement carries the SALE LINE's id. This is the property the
// whole design rests on: COGS joins SALE movements by sale_item_id, so a menu
// item's food cost is real without analytics knowing recipes exist.
func TestCompleteSale_CompositeMovementsLinkToTheSaleLine(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)

	rice := seedProduct(t, db, "ing-rice2", "Beras", 0)
	seedStock(t, db, rice, ownerID, 1000)
	nasgor := seedKindProduct(t, db, "menu-2", "Nasi Goreng", 20000, compositeKind)
	seedRecipe(t, db, nasgor, rice, 250)

	saleID := startDraft(t, svc, ctx)
	add, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 2,
	}))
	require.NoError(t, err)
	lineID := add.Msg.Sale.Items[0].Id

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 40000,
	}))
	require.NoError(t, err)

	var movements []model.StockMovement
	require.NoError(t, db.Where("type = ? AND sale_item_id = ?", "SALE", lineID).Find(&movements).Error)
	require.NotEmpty(t, movements, "the ingredient deduction must be attributed to the sale line")

	var total int32
	for _, m := range movements {
		total += m.Qty
		// Every movement is against an INGREDIENT batch, never the menu item.
		var b model.Batch
		require.NoError(t, db.Where("id = ?", m.BatchID).First(&b).Error)
		require.Equal(t, rice, b.ProductID)
	}
	require.Equal(t, int32(-500), total) // 2 portions x 250 g
}

// A shortfall in ONE ingredient fails the whole completion, and the rollback
// leaves both the sale and every other ingredient untouched — a half-cooked
// order must never reach the ledger.
func TestCompleteSale_CompositeInsufficientIngredientRollsBack(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)

	rice := seedProduct(t, db, "ing-rice3", "Beras", 0)
	egg := seedProduct(t, db, "ing-egg3", "Telur", 0)
	seedStock(t, db, rice, ownerID, 10_000) // plenty
	seedStock(t, db, egg, ownerID, 1)       // only one egg

	nasgor := seedKindProduct(t, db, "menu-3", "Nasi Goreng", 20000, compositeKind)
	seedRecipe(t, db, nasgor, rice, 200)
	seedRecipe(t, db, nasgor, egg, 1)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 3, // needs 3 eggs, has 1
	}))
	require.NoError(t, err)

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 60000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// Nothing consumed, sale still open.
	require.Equal(t, int64(10_000), stockOnHand(t, db, rice))
	require.Equal(t, int64(1), stockOnHand(t, db, egg))
	got, err := svc.GetSale(ctx, connect.NewRequest(&posifacev1.GetSaleRequest{Id: saleID}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_DRAFT, got.Msg.Sale.Status)
}

// A SERVICE line prices and sells but consumes nothing — and, crucially, does not
// fail for want of stock it was never supposed to have.
func TestCompleteSale_ServiceProductConsumesNothing(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	fee := seedKindProduct(t, db, "svc-corkage", "Corkage", 15000, serviceKind)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: fee, Qty: 1,
	}))
	require.NoError(t, err)

	resp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 15000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, resp.Msg.Sale.Status)

	var n int64
	require.NoError(t, db.Model(&model.StockMovement{}).
		Where("type = ?", "SALE").Count(&n).Error)
	require.Zero(t, n, "a service line must post no stock movements")
}

// A mixed cart resolves per line: the stocked item consumes itself, the composite
// consumes its ingredients, and both land in the same completed sale.
func TestCompleteSale_MixedCartResolvesPerLine(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)

	drink := seedProduct(t, db, "sku-teh", "Teh Botol", 5000)
	seedStock(t, db, drink, ownerID, 20)
	rice := seedProduct(t, db, "ing-rice4", "Beras", 0)
	seedStock(t, db, rice, ownerID, 1000)
	nasgor := seedKindProduct(t, db, "menu-4", "Nasi Goreng", 20000, compositeKind)
	seedRecipe(t, db, nasgor, rice, 200)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 2,
	}))
	require.NoError(t, err)
	_, err = svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: drink, Qty: 2,
	}))
	require.NoError(t, err)

	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 50000,
	}))
	require.NoError(t, err)

	require.Equal(t, int64(600), stockOnHand(t, db, rice)) // 1000 - 2x200
	require.Equal(t, int64(18), stockOnHand(t, db, drink)) // 20 - 2
}

// Refunding a composite sale with restock returns the INGREDIENTS, not the menu
// item (which has no batches to return anything to).
//
// This falls out of restockSaleItems mirroring the actual SALE movements rather
// than re-deriving from sale_items.product_id — the same property that makes
// COGS work. It is non-obvious enough that a future "simplification" could break
// it silently, leaving refunded food cost stranded, so it is pinned here.
func TestRefundSale_CompositeReturnsIngredients(t *testing.T) {
	t.Parallel()
	svc, ctx, db, ownerID := newSaleSvc(t)

	rice := seedProduct(t, db, "ing-rice-r", "Beras", 0)
	egg := seedProduct(t, db, "ing-egg-r", "Telur", 0)
	seedStock(t, db, rice, ownerID, 1000)
	seedStock(t, db, egg, ownerID, 10)
	nasgor := seedKindProduct(t, db, "menu-refund", "Nasi Goreng", 25000, compositeKind)
	seedRecipe(t, db, nasgor, rice, 200)
	seedRecipe(t, db, nasgor, egg, 1)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: nasgor, Qty: 2,
	}))
	require.NoError(t, err)
	_, err = svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 50000,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(600), stockOnHand(t, db, rice))
	require.Equal(t, int64(8), stockOnHand(t, db, egg))

	_, err = svc.RefundSale(ctx, connect.NewRequest(&posifacev1.RefundSaleRequest{
		SaleId: saleID, Restock: true, Reason: "wrong order",
	}))
	require.NoError(t, err)

	// Both ingredients are whole again — the dish itself never held stock.
	require.Equal(t, int64(1000), stockOnHand(t, db, rice))
	require.Equal(t, int64(10), stockOnHand(t, db, egg))
	require.Equal(t, int64(0), stockOnHand(t, db, nasgor))
}

// A COMPOSITE whose recipe is still empty consumes nothing and completes. Better
// than stranding a shop mid-service over a catalog gap; the product list surfaces
// it as 0 buildable so the omission is visible.
func TestCompleteSale_CompositeWithoutRecipeCompletes(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	dish := seedKindProduct(t, db, "menu-empty", "Menu Baru", 10000, compositeKind)

	saleID := startDraft(t, svc, ctx)
	_, err := svc.AddItem(ctx, connect.NewRequest(&posifacev1.AddItemRequest{
		SaleId: saleID, ProductId: dish, Qty: 1,
	}))
	require.NoError(t, err)

	resp, err := svc.CompleteSale(ctx, connect.NewRequest(&posifacev1.CompleteSaleRequest{
		SaleId: saleID, PaymentSource: posifacev1.PaymentSource_PAYMENT_SOURCE_CASH, PaidAmount: 10000,
	}))
	require.NoError(t, err)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_COMPLETED, resp.Msg.Sale.Status)
}
