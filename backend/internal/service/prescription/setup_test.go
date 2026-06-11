package prescription_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/model"
	prescriptionsvc "github.com/justmart/backend/internal/service/prescription"
	"github.com/justmart/backend/internal/service/servicetest"
)

// uniq yields a process-wide unique suffix so seeded SKUs/customers never
// collide, even across parallel tests or repeated createRx calls in one test.
var seedSeq atomic.Int64

func uniq() string { return fmt.Sprintf("%d", seedSeq.Add(1)) }

// rxEnv bundles the fixtures every prescription test needs: a live service, a
// seeded owner (the FK target for created_by), and an OWNER-authenticated ctx.
type rxEnv struct {
	svc     *prescriptionsvc.PrescriptionService
	db      *gorm.DB
	ownerID string
	ctx     context.Context
}

func newRxEnv(t *testing.T) rxEnv {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	return rxEnv{
		svc:     prescriptionsvc.NewPrescriptionService(db),
		db:      db,
		ownerID: ownerID,
		ctx:     servicetest.OwnerCtx(context.Background(), ownerID),
	}
}

func seedCustomer(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	c := model.Customer{Name: name}
	require.NoError(t, db.Create(&c).Error)
	require.NotEmpty(t, c.ID)
	return c.ID
}

func seedProduct(t *testing.T, db *gorm.DB, sku, name string, rxRequired bool) string {
	t.Helper()
	p := model.Product{
		SKU:                  sku,
		Name:                 name,
		Unit:                 "tablet",
		UnitPrice:            1000,
		PrescriptionRequired: rxRequired,
		Active:               true,
	}
	require.NoError(t, db.Create(&p).Error)
	require.NotEmpty(t, p.ID)
	return p.ID
}

// createRx seeds a customer + product and creates a single-line prescription via
// the service, returning the created proto. The default 90d expiry keeps it
// ACTIVE for assertions.
func createRx(t *testing.T, env rxEnv, qty int32) *prescriptionifacev1.Prescription {
	t.Helper()
	u := uniq()
	custID := seedCustomer(t, env.db, "Patient "+u)
	prodID := seedProduct(t, env.db, "SKU-"+u, "Drug "+u, true)
	resp, err := env.svc.CreatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		CustomerId: custID,
		IssuerName: "dr. Sutomo",
		IssuedAt:   "2026-06-01",
		Items: []*prescriptionifacev1.PrescriptionItemInput{
			{ProductId: prodID, PrescribedQty: qty, DosageInstructions: "3x1"},
		},
	}))
	require.NoError(t, err)
	return resp.Msg.Prescription
}
