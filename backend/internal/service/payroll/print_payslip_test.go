package payroll_test

import (
	"bytes"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/config"
)

// fakePusher captures the payload a connector would print.
type fakePusher struct {
	deviceID, printerName string
	payload               []byte
}

func (f *fakePusher) Push(deviceID, printerName string, payload []byte) (string, error) {
	f.deviceID = deviceID
	f.printerName = printerName
	f.payload = payload
	return "job-1", nil
}

func TestPrintPayslip_ConnectorReceivesBytes(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	createEmployee(t, env, "Budi Santoso", 5_000_000)
	run := createRun(t, env, 2026, 6)
	slip := run.Payslips[0]

	fake := &fakePusher{}
	env.pay.SetConnector(config.Connector{Mode: "connector"}, fake)

	resp, err := env.pay.PrintPayslip(env.ctx, connect.NewRequest(&payrollifacev1.PrintPayslipRequest{
		PayslipId: slip.Id,
	}))
	require.NoError(t, err)
	require.Positive(t, resp.Msg.BytesSent)
	require.NotEmpty(t, fake.payload)
	// The rendered slip carries the employee name and the net amount.
	require.True(t, bytes.Contains(fake.payload, []byte("Budi Santoso")))
	require.True(t, bytes.Contains(fake.payload, []byte("SLIP GAJI")))
	// No BPJS enrollment -> net equals the 5,000,000 base salary.
	require.True(t, bytes.Contains(fake.payload, []byte("5.000.000")))
}

func TestPayrollSettings_RoundTrip(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)

	// Defaults are returned even before any save.
	got, err := env.pay.GetPayrollSettings(env.ctx, connect.NewRequest(&payrollifacev1.GetPayrollSettingsRequest{}))
	require.NoError(t, err)
	require.Contains(t, got.Msg.BpjsConfigJson, "kesehatan")
	require.Contains(t, got.Msg.Pph21ConfigJson, "TER_MONTHLY")

	// Invalid JSON is rejected.
	_, err = env.pay.SetPayrollSettings(env.ctx, connect.NewRequest(&payrollifacev1.SetPayrollSettingsRequest{
		BpjsConfigJson: "nope", Pph21ConfigJson: got.Msg.Pph21ConfigJson,
	}))
	require.Error(t, err)
	require.Equal(t, "payroll.config_invalid", tokenOf(t, err))

	// A valid round-trip persists.
	newBpjs := `{"kesehatan":{"enabled":true,"emp_bps":150,"er_bps":400,"wage_cap":12000000,"basis":"gross"}}`
	set, err := env.pay.SetPayrollSettings(env.ctx, connect.NewRequest(&payrollifacev1.SetPayrollSettingsRequest{
		BpjsConfigJson: newBpjs, Pph21ConfigJson: got.Msg.Pph21ConfigJson,
	}))
	require.NoError(t, err)
	require.Contains(t, set.Msg.BpjsConfigJson, "150")
}
