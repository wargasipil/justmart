package batch_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	batchsvc "github.com/justmart/backend/internal/service/batch"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
)

type expiryEnv struct {
	db      *gorm.DB
	svc     *batchsvc.BatchService
	ctx     context.Context
	ownerID string
}

func newExpiryEnv(t *testing.T) *expiryEnv {
	t.Helper()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg)
	return &expiryEnv{
		db:      gormDB,
		svc:     batchsvc.NewBatchService(gormDB),
		ctx:     servicetest.OwnerCtx(context.Background(), ownerID),
		ownerID: ownerID,
	}
}

// lot creates a batch with the given expiry + source and returns its id.
func (e *expiryEnv) lot(t *testing.T, sku, expiry string, src inventoryifacev1.ExpirySource) string {
	t.Helper()
	prodID := seedProduct(t, e.db, sku, sku+" product")
	resp, err := e.svc.CreateBatch(e.ctx, connect.NewRequest(&inventoryifacev1.CreateBatchRequest{
		ProductId:       prodID,
		ExpiryDate:      expiry,
		ExpirySource:    src,
		InitialQuantity: 10,
	}))
	require.NoError(t, err)
	return resp.Msg.Batch.Id
}

func (e *expiryEnv) set(batchID, expiry, reason string, src inventoryifacev1.ExpirySource) (*inventoryifacev1.Batch, error) {
	resp, err := e.svc.SetBatchExpiry(e.ctx, connect.NewRequest(&inventoryifacev1.SetBatchExpiryRequest{
		BatchId:      batchID,
		ExpiryDate:   expiry,
		ExpirySource: src,
		Reason:       reason,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Batch, nil
}

// A typo fixed after receiving: the date moves, the cost and quantity don't,
// and the history keeps what it used to say, who changed it and why.
func TestSetBatchExpiry_CorrectsDateAndKeepsHistory(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	id := e.lot(t, "SE-1", "2025-03-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, e.db.Model(&model.Batch{}).Where("id = ?", id).Update("cost_price", 1234).Error)

	got, err := e.set(id, "2027-03-31", "typo when received", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_UNSPECIFIED)
	require.NoError(t, err)
	require.Equal(t, "2027-03-31", got.ExpiryDate)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED, got.ExpirySource)
	require.Equal(t, int64(1234), got.CostPrice, "only the expiry changes")
	require.Equal(t, int64(10), got.CurrentQuantity)

	var rows []model.BatchExpiryChange
	require.NoError(t, e.db.Where("batch_id = ?", id).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "2025-03-31", rows[0].OldExpiryDate.Format(common.DateLayout))
	require.Equal(t, "2027-03-31", rows[0].NewExpiryDate.Format(common.DateLayout))
	require.Equal(t, "typo when received", rows[0].Reason)
	require.Equal(t, e.ownerID, rows[0].ChangedBy)
}

// Re-saving a product default unchanged is how a shelf check confirms it: the
// date stays, the source becomes ENTERED, and that is recorded.
func TestSetBatchExpiry_ConfirmsADefault(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	id := e.lot(t, "SE-2", "2028-10-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT)

	got, err := e.set(id, "2028-10-31", "checked the pack", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED, got.ExpirySource)

	var row model.BatchExpiryChange
	require.NoError(t, e.db.Where("batch_id = ?", id).First(&row).Error)
	require.Equal(t, common.ExpirySourceDefault, row.OldExpirySource)
	require.Equal(t, common.ExpirySourceEntered, row.NewExpirySource)
}

// A dated lot can be switched to "does not expire" and back.
func TestSetBatchExpiry_NoExpiryRoundTrip(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	id := e.lot(t, "SE-3", "2026-12-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)

	got, err := e.set(id, "", "it is a plastic bucket", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)
	require.NoError(t, err)
	require.Equal(t, inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE, got.ExpirySource)
	require.Equal(t, "2099-12-31", got.ExpiryDate)

	got, err = e.set(id, "2027-01-31", "it was not", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.NoError(t, err)
	require.Equal(t, "2027-01-31", got.ExpiryDate)
}

func TestSetBatchExpiry_Refusals(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	id := e.lot(t, "SE-4", "2027-05-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)

	_, err := e.set(id, "2027-06-30", "  ", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.reason_required")

	_, err = e.set(id, "2027-05-31", "same", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.expiry_unchanged")

	_, err = e.set(id, "2027-06-30", "guess", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_DEFAULT)
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.expiry_source_invalid")

	_, err = e.set(id, "31/06/2027", "bad", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.expiry_invalid")

	_, err = e.set("", "2027-06-30", "who", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	requireBatchToken(t, err, connect.CodeInvalidArgument, "batch.batch_required")

	_, err = e.set("00000000-0000-0000-0000-000000000000", "2027-06-30", "missing",
		inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	// None of the refusals wrote history.
	var n int64
	require.NoError(t, e.db.Model(&model.BatchExpiryChange{}).Where("batch_id = ?", id).Count(&n).Error)
	require.Zero(t, n)
}

func TestSetBatchExpiry_PharmacyRxMustExpire(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	id := e.lot(t, "SE-5", "2027-05-31", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_ENTERED)
	var b model.Batch
	require.NoError(t, e.db.First(&b, "id = ?", id).Error)
	require.NoError(t, e.db.Model(&model.Product{}).Where("id = ?", b.ProductID).
		Update("prescription_required", true).Error)
	require.NoError(t, common.SetBussinessType(e.ctx, e.db, common.BussinessTypePharmacyShop))

	_, err := e.set(id, "", "no", inventoryifacev1.ExpirySource_EXPIRY_SOURCE_NONE)
	requireBatchToken(t, err, connect.CodeFailedPrecondition, "batch.expiry_required")
}

func TestSetBatchExpiry_Unauthenticated(t *testing.T) {
	t.Parallel()
	e := newExpiryEnv(t)
	_, err := e.svc.SetBatchExpiry(context.Background(), connect.NewRequest(&inventoryifacev1.SetBatchExpiryRequest{
		BatchId: "x", ExpiryDate: "2027-01-31", Reason: "r",
	}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
