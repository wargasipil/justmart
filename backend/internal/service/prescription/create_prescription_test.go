package prescription_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	prescriptionifacev1 "github.com/justmart/backend/gen/prescription_iface/v1"
)

func TestCreatePrescription_HappyPath(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	custID := seedCustomer(t, env.db, "Budi")
	prodID := seedProduct(t, env.db, "AMOX-500", "Amoxicillin 500mg", true)

	resp, err := env.svc.CreatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		CustomerId: custID,
		IssuerName: "dr. Sutomo",
		IssuedAt:   "2026-06-01",
		Items: []*prescriptionifacev1.PrescriptionItemInput{
			{ProductId: prodID, PrescribedQty: 10, DosageInstructions: "3x1"},
		},
	}))
	require.NoError(t, err)
	rx := resp.Msg.Prescription
	require.NotNil(t, rx)
	require.NotEmpty(t, rx.Id)
	require.NotEmpty(t, rx.RxNo)                 // RX-YYYY-NNNN assigned
	require.Equal(t, custID, rx.CustomerId)
	require.Equal(t, "ACTIVE", rx.Status)        // computed
	require.Equal(t, "2026-06-01", rx.IssuedAt)
	require.Equal(t, "2026-08-30", rx.ExpiresAt) // issued + 90d default
	require.Len(t, rx.Items, 1)
	require.Equal(t, prodID, rx.Items[0].ProductId)
	require.Equal(t, int32(10), rx.Items[0].PrescribedQty)
	require.Equal(t, int32(0), rx.Items[0].DispensedQty)
}

func TestCreatePrescription_WithFeeAndPatientInfo(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	custID := seedCustomer(t, env.db, "Budi")
	prodID := seedProduct(t, env.db, "AMOX-500", "Amoxicillin 500mg", true)

	resp, err := env.svc.CreatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		CustomerId:     custID,
		IssuerName:     "dr. Sutomo",
		IssuedAt:       "2026-06-01",
		BiayaJasa:      15000,
		PatientAge:     7,
		PatientWeight:  "20 kg",
		PatientAllergy: "penicillin",
		Items: []*prescriptionifacev1.PrescriptionItemInput{
			{ProductId: prodID, PrescribedQty: 10},
		},
	}))
	require.NoError(t, err)
	rx := resp.Msg.Prescription
	require.Equal(t, int64(15000), rx.BiayaJasa)
	require.Equal(t, int32(7), rx.PatientAge)
	require.Equal(t, "20 kg", rx.PatientWeight)
	require.Equal(t, "penicillin", rx.PatientAllergy)

	// Round-trips through Get too.
	got, err := env.svc.GetPrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.GetPrescriptionRequest{Id: rx.Id}))
	require.NoError(t, err)
	require.Equal(t, int64(15000), got.Msg.Prescription.BiayaJasa)
}

func TestCreatePrescription_RejectsNegativeFee(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	custID := seedCustomer(t, env.db, "Citra")
	prodID := seedProduct(t, env.db, "PARA-500", "Paracetamol 500mg", true)

	_, err := env.svc.CreatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		CustomerId: custID,
		IssuerName: "dr. Sutomo",
		IssuedAt:   "2026-06-01",
		BiayaJasa:  -1,
		Items:      []*prescriptionifacev1.PrescriptionItemInput{{ProductId: prodID, PrescribedQty: 1}},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreatePrescription_RequiresCustomer(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	prodID := seedProduct(t, env.db, "PARA-500", "Paracetamol 500mg", false)

	_, err := env.svc.CreatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		CustomerId: "", // missing -> InvalidArgument
		IssuerName: "dr. Sutomo",
		IssuedAt:   "2026-06-01",
		Items: []*prescriptionifacev1.PrescriptionItemInput{
			{ProductId: prodID, PrescribedQty: 5},
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreatePrescription_RequiresItems(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)
	custID := seedCustomer(t, env.db, "Citra")

	_, err := env.svc.CreatePrescription(env.ctx, connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		CustomerId: custID,
		IssuerName: "dr. Sutomo",
		IssuedAt:   "2026-06-01",
		Items:      nil, // no lines -> InvalidArgument
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreatePrescription_Unauthenticated(t *testing.T) {
	t.Parallel()
	env := newRxEnv(t)

	_, err := env.svc.CreatePrescription(context.Background(), connect.NewRequest(&prescriptionifacev1.CreatePrescriptionRequest{
		IssuerName: "dr. Sutomo",
		IssuedAt:   "2026-06-01",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
