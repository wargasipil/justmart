package prescription_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
)

func TestListPrescriptions_All(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	createRx(t, env, 10)
	createRx(t, env, 5)

	resp, err := env.svc.ListPrescriptions(env.ctx, connect.NewRequest(&prescriptionifacev1.ListPrescriptionsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Total)
	require.Len(t, resp.Msg.Prescriptions, 2)
}

func TestListPrescriptions_FilterByCustomer(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	first := createRx(t, env, 10)
	createRx(t, env, 5) // different customer

	resp, err := env.svc.ListPrescriptions(env.ctx, connect.NewRequest(&prescriptionifacev1.ListPrescriptionsRequest{
		CustomerId: first.CustomerId,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Total)
	require.Len(t, resp.Msg.Prescriptions, 1)
	require.Equal(t, first.Id, resp.Msg.Prescriptions[0].Id)
}

func TestListPrescriptions_FilterByComputedStatus(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	createRx(t, env, 10) // ACTIVE

	// ACTIVE filter keeps it; VOIDED filter drops it (total still counts base rows).
	active, err := env.svc.ListPrescriptions(env.ctx, connect.NewRequest(&prescriptionifacev1.ListPrescriptionsRequest{
		Status: "ACTIVE",
	}))
	require.NoError(t, err)
	require.Len(t, active.Msg.Prescriptions, 1)

	voided, err := env.svc.ListPrescriptions(env.ctx, connect.NewRequest(&prescriptionifacev1.ListPrescriptionsRequest{
		Status: "VOIDED",
	}))
	require.NoError(t, err)
	require.Len(t, voided.Msg.Prescriptions, 0)
}
