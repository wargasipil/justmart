package priceagreement_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func TestListPriceAgreements_Filters(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	supA := seedSupplier(t, db, "LS-A")
	supB := seedSupplier(t, db, "LS-B")
	paracetamol, _, pBox := seedProductWithUnits(t, db, "ls-para")
	amox, _, aBox := seedProductWithUnits(t, db, "ls-amox")

	mk := func(sup, prod, unit string, price int64) {
		_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
			SupplierId: sup, ProductId: prod, ProductUnitId: unit, Price: price,
		}))
		require.NoError(t, err)
	}
	mk(supA, paracetamol, pBox, 100)
	mk(supA, amox, aBox, 200)
	mk(supB, paracetamol, pBox, 110)

	list := func(req *inventoryifacev1.ListPriceAgreementsRequest) *inventoryifacev1.ListPriceAgreementsResponse {
		r, err := svc.ListPriceAgreements(ctx(), connect.NewRequest(req))
		require.NoError(t, err)
		return r.Msg
	}

	// All active.
	require.Equal(t, int32(3), list(&inventoryifacev1.ListPriceAgreementsRequest{}).Total)
	// By supplier.
	require.Equal(t, int32(2), list(&inventoryifacev1.ListPriceAgreementsRequest{SupplierId: supA}).Total)
	// By product (paracetamol is at two suppliers).
	require.Equal(t, int32(2), list(&inventoryifacev1.ListPriceAgreementsRequest{ProductId: paracetamol}).Total)
	// By supplier + product.
	require.Equal(t, int32(1), list(&inventoryifacev1.ListPriceAgreementsRequest{SupplierId: supA, ProductId: amox}).Total)
	// Query by product sku.
	require.Equal(t, int32(1), list(&inventoryifacev1.ListPriceAgreementsRequest{Query: "ls-amox"}).Total)
	// Query by supplier code.
	require.Equal(t, int32(1), list(&inventoryifacev1.ListPriceAgreementsRequest{Query: "LS-B"}).Total)
}

func TestListPriceAgreements_ValidityFilter(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "VAL-A")
	pExp, _, uExp := seedProductWithUnits(t, db, "val-exp")
	pUp, _, uUp := seedProductWithUnits(t, db, "val-up")
	pCurWin, _, uCurWin := seedProductWithUnits(t, db, "val-cur")
	pCurOpen, _, uCurOpen := seedProductWithUnits(t, db, "val-open")

	const layout = "2006-01-02"
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Format(layout)
	tomorrow := now.AddDate(0, 0, 1).Format(layout)
	lastYear := now.AddDate(-1, 0, 0).Format(layout)
	nextYear := now.AddDate(1, 0, 0).Format(layout)

	mk := func(prod, unit, vf, vu string) {
		_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
			SupplierId: sup, ProductId: prod, ProductUnitId: unit, Price: 100, ValidFrom: vf, ValidUntil: vu,
		}))
		require.NoError(t, err)
	}
	mk(pExp, uExp, lastYear, yesterday)     // expired (ended yesterday)
	mk(pUp, uUp, tomorrow, nextYear)        // upcoming (starts tomorrow)
	mk(pCurWin, uCurWin, yesterday, tomorrow) // current (windowed)
	mk(pCurOpen, uCurOpen, "", "")          // current (open-ended, no dates)

	total := func(v inventoryifacev1.PriceAgreementValidity) int32 {
		r, err := svc.ListPriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.ListPriceAgreementsRequest{
			SupplierId: sup, Validity: v,
		}))
		require.NoError(t, err)
		return r.Msg.Total
	}

	require.Equal(t, int32(4), total(inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_UNSPECIFIED))
	require.Equal(t, int32(2), total(inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_CURRENT)) // windowed + open-ended
	require.Equal(t, int32(1), total(inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_EXPIRED))
	require.Equal(t, int32(1), total(inventoryifacev1.PriceAgreementValidity_PRICE_AGREEMENT_VALIDITY_UPCOMING))
}

func TestListPriceAgreements_ExcludesArchivedByDefault(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "LS-ARCH")
	prod, _, box := seedProductWithUnits(t, db, "ls-arch")

	created, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
	}))
	require.NoError(t, err)
	_, err = svc.ArchivePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.ArchivePriceAgreementRequest{Id: created.Msg.Agreement.Id}))
	require.NoError(t, err)

	def, err := svc.ListPriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.ListPriceAgreementsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(0), def.Msg.Total)

	incl, err := svc.ListPriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.ListPriceAgreementsRequest{IncludeInactive: true}))
	require.NoError(t, err)
	require.Equal(t, int32(1), incl.Msg.Total)
}
