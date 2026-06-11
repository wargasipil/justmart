package prescription_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
)

func TestGetPrescription_HappyPath(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	created := createRx(t, env, 10)

	resp, err := env.svc.GetPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.GetPrescriptionRequest{
		Id: created.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, created.Id, resp.Msg.Prescription.Id)
	require.Equal(t, created.RxNo, resp.Msg.Prescription.RxNo)
	require.Equal(t, "ACTIVE", resp.Msg.Prescription.Status)
	require.Len(t, resp.Msg.Prescription.Items, 1)
}

func TestGetPrescription_NotFound(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)

	_, err := env.svc.GetPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.GetPrescriptionRequest{
		Id: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
