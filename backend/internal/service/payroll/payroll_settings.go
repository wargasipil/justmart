package payroll

import (
	"context"
	"encoding/json"
	"strings"

	"connectrpc.com/connect"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetPayrollSettings returns the stored BPJS + PPh21 config JSON, falling back to
// the baked provisional defaults when a key is unset (so a fresh shop shows the
// editable defaults rather than blanks).
func (s *PayrollService) GetPayrollSettings(
	ctx context.Context,
	_ *connect.Request[payrollifacev1.GetPayrollSettingsRequest],
) (*connect.Response[payrollifacev1.GetPayrollSettingsResponse], error) {
	bpjs, pph21, err := common.GetPayrollConfig(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if strings.TrimSpace(bpjs) == "" {
		bpjs = DefaultBPJSConfigJSON()
	}
	if strings.TrimSpace(pph21) == "" {
		pph21 = DefaultPPh21ConfigJSON()
	}
	return connect.NewResponse(&payrollifacev1.GetPayrollSettingsResponse{
		BpjsConfigJson:  bpjs,
		Pph21ConfigJson: pph21,
	}), nil
}

// SetPayrollSettings validates that both blobs parse as the expected config
// shapes, then persists them.
func (s *PayrollService) SetPayrollSettings(
	ctx context.Context,
	req *connect.Request[payrollifacev1.SetPayrollSettingsRequest],
) (*connect.Response[payrollifacev1.SetPayrollSettingsResponse], error) {
	bpjs := strings.TrimSpace(req.Msg.BpjsConfigJson)
	pph21 := strings.TrimSpace(req.Msg.Pph21ConfigJson)
	if bpjs == "" || pph21 == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.config_required")
	}
	var bc BPJSConfig
	if err := json.Unmarshal([]byte(bpjs), &bc); err != nil {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.config_invalid")
	}
	var pc PPh21Config
	if err := json.Unmarshal([]byte(pph21), &pc); err != nil {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.config_invalid")
	}
	if err := common.SetPayrollConfig(ctx, s.db, bpjs, pph21); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&payrollifacev1.SetPayrollSettingsResponse{
		BpjsConfigJson:  bpjs,
		Pph21ConfigJson: pph21,
	}), nil
}
