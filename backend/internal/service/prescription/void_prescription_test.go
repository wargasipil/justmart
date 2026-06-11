package prescription_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
)

func TestVoidPrescription_HappyPath(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	created := createRx(t, env, 10)

	resp, err := env.svc.VoidPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.VoidPrescriptionRequest{
		Id: created.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, "VOIDED", resp.Msg.Prescription.Status)
}

func TestVoidPrescription_Idempotent(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	created := createRx(t, env, 10)

	_, err := env.svc.VoidPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.VoidPrescriptionRequest{Id: created.Id}))
	require.NoError(t, err)
	// Second void is a no-op, still returns VOIDED.
	resp, err := env.svc.VoidPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.VoidPrescriptionRequest{Id: created.Id}))
	require.NoError(t, err)
	require.Equal(t, "VOIDED", resp.Msg.Prescription.Status)
}

func TestVoidPrescription_NotFound(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)

	_, err := env.svc.VoidPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.VoidPrescriptionRequest{
		Id: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
