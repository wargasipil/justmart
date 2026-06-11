package prescription_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestUpdatePrescription_HappyPath(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	created := createRx(t, env, 10)
	newProd := seedProduct(t, env.db, "NEW-SKU", "Replacement drug", true)

	resp, err := env.svc.UpdatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.UpdatePrescriptionRequest{
		Id:         created.Id,
		IssuerName: "dr. Wati",
		IssuedAt:   "2026-06-02",
		ExpiresAt:  "2026-07-02",
		Note:       "updated",
		Items: []*prescriptionifacev1.PrescriptionItemInput{
			{ProductId: newProd, PrescribedQty: 20, DosageInstructions: "2x1"},
		},
	}))
	require.NoError(t, err)
	rx := resp.Msg.Prescription
	require.Equal(t, "dr. Wati", rx.IssuerName)
	require.Equal(t, "2026-06-02", rx.IssuedAt)
	require.Equal(t, "2026-07-02", rx.ExpiresAt)
	require.Len(t, rx.Items, 1) // full replace
	require.Equal(t, newProd, rx.Items[0].ProductId)
	require.Equal(t, int32(20), rx.Items[0].PrescribedQty)
}

func TestUpdatePrescription_BlockedAfterDispensing(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	created := createRx(t, env, 10)

	// Simulate a partial dispense directly on the line.
	require.NoError(t, env.db.Model(&model.PrescriptionItem{}).
		Where("prescription_id = ?", created.Id).
		Update("dispensed_qty", 3).Error)

	_, err := env.svc.UpdatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.UpdatePrescriptionRequest{
		Id:         created.Id,
		IssuerName: "dr. Wati",
		IssuedAt:   "2026-06-02",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestUpdatePrescription_BlockedWhenVoided(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	created := createRx(t, env, 10)
	_, err := env.svc.VoidPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.VoidPrescriptionRequest{Id: created.Id}))
	require.NoError(t, err)

	_, err = env.svc.UpdatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.UpdatePrescriptionRequest{
		Id:         created.Id,
		IssuerName: "dr. Wati",
		IssuedAt:   "2026-06-02",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
