package product_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	productsvc "github.com/justmart/backend/internal/service/product"
	"github.com/justmart/backend/internal/service/servicetest"
)

// seedManufacturer inserts a pabrik directly -- the product package's tests must
// not depend on the manufacturer service to exercise a product RPC.
func seedManufacturer(t *testing.T, db *gorm.DB, code, name string, active bool) string {
	t.Helper()
	m := model.Manufacturer{Code: code, Name: name, Active: true}
	require.NoError(t, db.Create(&m).Error)
	// Archived rows are made by a SECOND write on purpose: Active carries
	// `default:true`, so GORM omits a false on INSERT and the row would come
	// back ACTIVE -- silently turning an archived-maker test into a happy path.
	if !active {
		require.NoError(t, db.Model(&m).Update("active", false).Error)
	}
	return m.ID
}

type mfrEnv struct {
	db  *gorm.DB
	svc *productsvc.ProductService
	ctx context.Context
}

func newMfrEnv(t *testing.T) *mfrEnv {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	return &mfrEnv{
		db:  gormDB,
		svc: productsvc.NewProductService(gormDB),
		ctx: servicetest.OwnerCtx(context.Background(), ownerID),
	}
}

func (e *mfrEnv) add(t *testing.T, productID, manufacturerID string) *inventoryifacev1.Product {
	t.Helper()
	resp, err := e.svc.AddProductManufacturer(e.ctx, connect.NewRequest(
		&inventoryifacev1.AddProductManufacturerRequest{
			ProductId: productID, ManufacturerId: manufacturerID,
		}))
	require.NoError(t, err)
	return resp.Msg.Product
}

func (e *mfrEnv) set(t *testing.T, productID string, ids []string, primary string) (*inventoryifacev1.Product, error) {
	t.Helper()
	resp, err := e.svc.SetProductManufacturers(e.ctx, connect.NewRequest(
		&inventoryifacev1.SetProductManufacturersRequest{
			ProductId: productID, ManufacturerIds: ids, PrimaryManufacturerId: primary,
		}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Product, nil
}

// approvedIDs reads a product's approved-source list straight from the table,
// so a test can assert what was STORED rather than what the enrich hoists.
func (e *mfrEnv) approvedIDs(t *testing.T, productID string) []string {
	t.Helper()
	var rows []model.ProductManufacturer
	require.NoError(t, e.db.Where("product_id = ?", productID).
		Order("created_at ASC, id ASC").Find(&rows).Error)
	out := make([]string, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].ManufacturerID)
	}
	return out
}

// CreateProduct is not one of the two narrow writers, but it can still set a
// pabrik -- so it has to maintain the same invariant. Without this the product
// points at a maker its own approved list does not contain, and the pabrik's
// product page (which lists by that table) never shows it.
func TestCreateProduct_ApprovesThePrimaryItSets(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	kalbe := seedManufacturer(t, e.db, "MFR-CP-1", "Kalbe Farma", true)

	resp, err := e.svc.CreateProduct(e.ctx, connect.NewRequest(&inventoryifacev1.CreateProductRequest{
		Sku: "SKU-CP-1", Name: "Paracetamol 500", Unit: "tablet", UnitPrice: 5000,
		ManufacturerId: kalbe,
	}))
	require.NoError(t, err)
	require.Equal(t, kalbe, resp.Msg.Product.ManufacturerId)
	require.Equal(t, []string{kalbe}, e.approvedIDs(t, resp.Msg.Product.Id),
		"the primary must be a member of its own set")
	require.Equal(t, []string{kalbe}, resp.Msg.Product.ManufacturerIds)
}

// Naming a NEW usual maker on the product form approves it too -- ADDITIVELY.
// A form holds one maker, so replacing the list here would silently drop every
// other approved source, which is the overwrite the split model exists to stop.
func TestUpdateProduct_ApprovesTheNewPrimaryWithoutDroppingSources(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	kalbe := seedManufacturer(t, e.db, "MFR-UP-1", "Kalbe Farma", true)
	dexa := seedManufacturer(t, e.db, "MFR-UP-2", "Dexa Medica", true)
	hexpharm := seedManufacturer(t, e.db, "MFR-UP-3", "Hexpharm Jaya", true)

	pid := seedProduct(t, e.svc, e.ctx, "SKU-UP-1", "Amoxsan", 7500)
	e.add(t, pid, kalbe)
	e.add(t, pid, dexa) // a second approved source, recorded off an invoice

	// The form switches the USUAL maker to a third, previously unlisted pabrik.
	out, err := e.svc.UpdateProduct(e.ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: pid, Name: "Amoxsan", Unit: "tablet", UnitPrice: 7500,
		ManufacturerId: hexpharm,
	}))
	require.NoError(t, err)
	require.Equal(t, hexpharm, out.Msg.Product.ManufacturerId)
	require.ElementsMatch(t, []string{kalbe, dexa, hexpharm}, e.approvedIDs(t, pid),
		"the earlier sources must survive a change of usual maker")
}

// Clearing the usual maker does NOT retract permission to source from them.
// "Who usually makes this" and "who may make this" are different facts, and
// removal is SetProductManufacturers' job, expressed by absence from a full set.
func TestUpdateProduct_ClearingThePrimaryKeepsTheApprovedList(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	kalbe := seedManufacturer(t, e.db, "MFR-UC-1", "Kalbe Farma", true)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-UC-1", "Bodrex", 3000)
	e.add(t, pid, kalbe)

	out, err := e.svc.UpdateProduct(e.ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: pid, Name: "Bodrex", Unit: "tablet", UnitPrice: 3000, ManufacturerId: "",
	}))
	require.NoError(t, err)
	require.Empty(t, out.Msg.Product.ManufacturerId, "the pointer clears")
	require.Equal(t, []string{kalbe}, e.approvedIDs(t, pid), "the list does not")
}

// An ARCHIVED pabrik cannot be assigned here either -- the product form is a
// third way in, and a rule only two of three writers enforce is not a rule.
func TestUpdateProduct_RejectsAnArchivedManufacturer(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	gone := seedManufacturer(t, e.db, "MFR-UA-1", "Gone Pharma", false)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-UA-1", "Promag", 4000)

	_, err := e.svc.UpdateProduct(e.ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: pid, Name: "Promag", Unit: "tablet", UnitPrice: 4000, ManufacturerId: gone,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, "manufacturer.archived", ce.Message())
	require.Empty(t, e.approvedIDs(t, pid), "a refused assignment writes nothing")
}

// ...but a product ALREADY pointing at a factory that has since been archived
// must stay editable in every other field. Only a CHANGE is checked.
func TestUpdateProduct_ReSavingAnArchivedPrimaryIsAllowed(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	maker := seedManufacturer(t, e.db, "MFR-UR-1", "Later Retired", true)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-UR-1", "Tolak Angin", 6000)
	e.add(t, pid, maker)
	require.NoError(t, e.db.Model(&model.Manufacturer{}).
		Where("id = ?", maker).Update("active", false).Error)

	out, err := e.svc.UpdateProduct(e.ctx, connect.NewRequest(&inventoryifacev1.UpdateProductRequest{
		Id: pid, Name: "Tolak Angin Cair", Unit: "sachet", UnitPrice: 6500,
		ManufacturerId: maker,
	}))
	require.NoError(t, err, "archiving a factory must not freeze every product it makes")
	require.Equal(t, "Tolak Angin Cair", out.Msg.Product.Name)
	require.Equal(t, maker, out.Msg.Product.ManufacturerId)
}

// The point of the whole model: one product, several approved makers.
func TestAddProductManufacturer_KeepsEveryApprovedSource(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-1", "Paracetamol 500", 5000)
	kalbe := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)
	dexa := seedManufacturer(t, e.db, "MFR-2", "Dexa Medica", true)

	// The FIRST source becomes the primary: a lone maker is the usual one.
	got := e.add(t, pid, kalbe)
	require.Equal(t, kalbe, got.ManufacturerId)
	require.Equal(t, []string{kalbe}, got.ManufacturerIds)

	// The second must NOT displace it -- this is exactly the overwrite the
	// single-column model used to perform on every restock from a new pabrik.
	got = e.add(t, pid, dexa)
	require.Equal(t, kalbe, got.ManufacturerId, "adding a source must not move the primary")
	require.ElementsMatch(t, []string{kalbe, dexa}, got.ManufacturerIds)
	require.Equal(t, kalbe, got.ManufacturerIds[0], "primary leads the list")
}

func TestAddProductManufacturer_IsIdempotent(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-2", "Amoxsan", 7500)
	mid := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)

	e.add(t, pid, mid)
	got := e.add(t, pid, mid)
	require.Equal(t, []string{mid}, got.ManufacturerIds)

	var n int64
	require.NoError(t, e.db.Model(&model.ProductManufacturer{}).
		Where("product_id = ?", pid).Count(&n).Error)
	require.EqualValues(t, 1, n, "re-adding an approved maker must not duplicate the row")
}

// The restock form calls this mid-order; it must touch nothing else.
func TestAddProductManufacturer_LeavesTheRestOfTheProductAlone(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-3", "Promag", 3000)
	mid := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)

	before, err := e.svc.GetProduct(e.ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)
	e.add(t, pid, mid)
	after, err := e.svc.GetProduct(e.ctx, connect.NewRequest(&inventoryifacev1.GetProductRequest{Id: pid}))
	require.NoError(t, err)

	require.Equal(t, before.Msg.Product.Name, after.Msg.Product.Name)
	require.Equal(t, before.Msg.Product.Sku, after.Msg.Product.Sku)
	require.Equal(t, before.Msg.Product.UnitPrice, after.Msg.Product.UnitPrice)
	require.Len(t, after.Msg.Product.Units, len(before.Msg.Product.Units))
}

func TestAddProductManufacturer_RejectsArchivedMaker(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-4", "Bodrex", 4000)
	gone := seedManufacturer(t, e.db, "MFR-9", "Gone Pharma", false)

	_, err := e.svc.AddProductManufacturer(e.ctx, connect.NewRequest(
		&inventoryifacev1.AddProductManufacturerRequest{ProductId: pid, ManufacturerId: gone}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, "manufacturer.archived", ce.Message())
}

func TestAddProductManufacturer_UnknownMakerIsNotFound(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-5", "Panadol", 6000)

	_, err := e.svc.AddProductManufacturer(e.ctx, connect.NewRequest(
		&inventoryifacev1.AddProductManufacturerRequest{
			ProductId: pid, ManufacturerId: "00000000-0000-0000-0000-000000000000",
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSetProductManufacturers_ReplacesTheListAndNamesThePrimary(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-6", "Paracetamol 500", 5000)
	a := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)
	b := seedManufacturer(t, e.db, "MFR-2", "Dexa Medica", true)
	c := seedManufacturer(t, e.db, "MFR-3", "Hexpharm Jaya", true)

	got, err := e.set(t, pid, []string{a, b}, b)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{a, b}, got.ManufacturerIds)
	require.Equal(t, b, got.ManufacturerId)

	// A removal is expressed by absence: `a` is gone, `c` arrives.
	got, err = e.set(t, pid, []string{b, c}, "")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{b, c}, got.ManufacturerIds)
	require.Equal(t, b, got.ManufacturerId, "a surviving primary is kept when none is named")
}

// Dropping the primary must not leave products.manufacturer_id dangling at a
// maker the product is no longer approved for.
func TestSetProductManufacturers_DroppingThePrimaryFallsBack(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-7", "Mixagrip", 4500)
	a := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)
	b := seedManufacturer(t, e.db, "MFR-2", "Dexa Medica", true)

	_, err := e.set(t, pid, []string{a, b}, a)
	require.NoError(t, err)

	got, err := e.set(t, pid, []string{b}, "")
	require.NoError(t, err)
	require.Equal(t, []string{b}, got.ManufacturerIds)
	require.Equal(t, b, got.ManufacturerId)

	// Clearing the list clears the pointer too, rather than dangling it.
	got, err = e.set(t, pid, nil, "")
	require.NoError(t, err)
	require.Empty(t, got.ManufacturerIds)
	require.Empty(t, got.ManufacturerId)

	var row model.Product
	require.NoError(t, e.db.Where("id = ?", pid).First(&row).Error)
	require.Nil(t, row.ManufacturerID, "the column must be NULL, not an empty string the FK would reject")
}

func TestSetProductManufacturers_PrimaryMustBeOnTheList(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-8", "Neozep", 3500)
	a := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)
	b := seedManufacturer(t, e.db, "MFR-2", "Dexa Medica", true)

	_, err := e.set(t, pid, []string{a}, b)
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, "manufacturer.primary_not_listed", ce.Message())
}

// Archiving a pabrik must not make every product it makes un-editable.
func TestSetProductManufacturers_KeepsAnArchivedMakerAlreadyOnTheList(t *testing.T) {
	t.Parallel()
	e := newMfrEnv(t)
	pid := seedProduct(t, e.svc, e.ctx, "SKU-MFR-9", "Ultraflu", 3800)
	live := seedManufacturer(t, e.db, "MFR-1", "Kalbe Farma", true)
	gone := seedManufacturer(t, e.db, "MFR-9", "Gone Pharma", false)

	// Approve it while it is still active, then retire the factory.
	_, err := e.set(t, pid, []string{live, gone}, live)
	require.Error(t, err, "an archived maker cannot be ADDED")

	// Seed the membership directly, the way an older, then-active row would have.
	require.NoError(t, e.db.Create(&model.ProductManufacturer{
		ProductID: pid, ManufacturerID: gone,
	}).Error)

	got, err := e.set(t, pid, []string{gone, live}, "")
	require.NoError(t, err, "re-saving a list that already holds an archived maker must not be refused")
	require.ElementsMatch(t, []string{live, gone}, got.ManufacturerIds)
}
