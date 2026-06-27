package priceagreement_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestCreatePriceAgreements_MultiLine(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUPB-A")
	prod, base, box := seedProductWithUnits(t, db, "pab-1")

	resp, err := svc.CreatePriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementsRequest{
		SupplierId: sup,
		Items: []*inventoryifacev1.PriceAgreementItemInput{
			{ProductId: prod, ProductUnitId: box, Price: 140000, ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", Note: "box"},
			{ProductId: prod, ProductUnitId: base, Price: 12000},
		},
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Agreements, 2)

	byUnit := map[string]*inventoryifacev1.PriceAgreement{}
	for _, a := range resp.Msg.Agreements {
		require.NotEmpty(t, a.Id)
		require.Equal(t, sup, a.SupplierId)
		require.True(t, a.Active)
		byUnit[a.ProductUnitId] = a
	}
	require.Equal(t, "box", byUnit[box].UnitName)
	require.Equal(t, int64(12), byUnit[box].UnitFactor)
	require.Equal(t, int64(140000), byUnit[box].Price)
	require.Equal(t, "2026-12-31", byUnit[box].ValidUntil)
	require.Equal(t, "pcs", byUnit[base].UnitName)
	require.Equal(t, int64(1), byUnit[base].UnitFactor)

	var count int64
	require.NoError(t, db.Model(&model.PriceAgreement{}).Where("supplier_id = ?", sup).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestCreatePriceAgreements_WithinRequestDuplicateRejected(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUPB-B")
	prod, _, box := seedProductWithUnits(t, db, "pab-2")

	_, err := svc.CreatePriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementsRequest{
		SupplierId: sup,
		Items: []*inventoryifacev1.PriceAgreementItemInput{
			{ProductId: prod, ProductUnitId: box, Price: 1000},
			{ProductId: prod, ProductUnitId: box, Price: 2000}, // same (product, unit)
		},
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "price_agreement.exists")

	// All-or-nothing: nothing was created.
	var count int64
	require.NoError(t, db.Model(&model.PriceAgreement{}).Where("supplier_id = ?", sup).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestCreatePriceAgreements_ExistingActiveDuplicateRejected(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUPB-C")
	prod, base, box := seedProductWithUnits(t, db, "pab-3")

	// Seed an existing active agreement for (sup, prod, box).
	_, err := svc.CreatePriceAgreement(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementRequest{
		SupplierId: sup, ProductId: prod, ProductUnitId: box, Price: 1000,
	}))
	require.NoError(t, err)

	// A batch whose first line is new but second collides with the existing one.
	_, err = svc.CreatePriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementsRequest{
		SupplierId: sup,
		Items: []*inventoryifacev1.PriceAgreementItemInput{
			{ProductId: prod, ProductUnitId: base, Price: 500}, // new
			{ProductId: prod, ProductUnitId: box, Price: 2000}, // duplicate of existing
		},
	}))
	requireToken(t, err, connect.CodeAlreadyExists, "price_agreement.exists")

	// All-or-nothing: only the pre-seeded row exists; the new line rolled back.
	var count int64
	require.NoError(t, db.Model(&model.PriceAgreement{}).Where("supplier_id = ?", sup).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestCreatePriceAgreements_EmptyItemsRejected(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUPB-D")

	_, err := svc.CreatePriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementsRequest{
		SupplierId: sup,
		Items:      nil,
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "price_agreement.items_required")
}

func TestCreatePriceAgreements_BadLineRollsBackAll(t *testing.T) {
	t.Parallel()
	svc, db := newSvc(t)
	sup := seedSupplier(t, db, "SUPB-E")
	prod, base, box := seedProductWithUnits(t, db, "pab-4")

	// First line valid; second (a different unit) has bad dates → whole batch
	// rolls back. (Distinct units so the within-request dedupe isn't what trips.)
	_, err := svc.CreatePriceAgreements(ctx(), connect.NewRequest(&inventoryifacev1.CreatePriceAgreementsRequest{
		SupplierId: sup,
		Items: []*inventoryifacev1.PriceAgreementItemInput{
			{ProductId: prod, ProductUnitId: box, Price: 1000},
			{ProductId: prod, ProductUnitId: base, Price: 1000, ValidFrom: "2026-12-31", ValidUntil: "2026-01-01"},
		},
	}))
	requireToken(t, err, connect.CodeInvalidArgument, "price_agreement.bad_dates")

	var count int64
	require.NoError(t, db.Model(&model.PriceAgreement{}).Where("supplier_id = ?", sup).Count(&count).Error)
	require.Equal(t, int64(0), count)
}
