package payroll_test

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

// tokenOf returns the stable token message of a connect error.
func tokenOf(t *testing.T, err error) string {
	t.Helper()
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "expected a connect error, got %v", err)
	return ce.Message()
}

func TestCreateEmployee_HappyPath(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	resp, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code:       "emp-001", // lowercased on input
		Name:       "Budi",
		Position:   "Kasir",
		BaseSalary: 5_000_000,
		PtkpStatus: "k1",
		JoinedAt:   "2026-01-15",
		Allowances: []*payrollifacev1.EmployeeAllowanceInput{
			{Label: "Transport", Amount: 500_000, Taxable: true},
			{Label: "Makan", Amount: 300_000, Taxable: false},
		},
	}))
	require.NoError(t, err)
	e := resp.Msg.Employee
	require.NotEmpty(t, e.Id)
	require.Equal(t, "EMP-001", e.Code) // uppercased
	require.Equal(t, "Budi", e.Name)
	require.Equal(t, int64(5_000_000), e.BaseSalary)
	require.Equal(t, "K1", e.PtkpStatus) // uppercased
	require.Equal(t, "2026-01-15", e.JoinedAt)
	require.True(t, e.Active)
	require.Len(t, e.Allowances, 2)
}

func TestCreateEmployee_RequiredFields(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "", Name: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.Equal(t, "payroll.required", tokenOf(t, err))
}

func TestCreateEmployee_CodeTaken(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-DUP", Name: "A", BaseSalary: 1000,
	}))
	require.NoError(t, err)
	_, err = env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "emp-dup", Name: "B", BaseSalary: 2000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	require.Equal(t, "payroll.code_taken", tokenOf(t, err))
}

func TestCreateEmployee_PtkpInvalid(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-X", Name: "X", PtkpStatus: "ZZ9",
	}))
	require.Error(t, err)
	require.Equal(t, "payroll.ptkp_invalid", tokenOf(t, err))
}

func TestCreateEmployee_UserLink(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	// Link to the seeded owner user.
	_, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-U1", Name: "Owner emp", UserId: env.ownerID, BaseSalary: 1000,
	}))
	require.NoError(t, err)

	// A second employee linking the same user is rejected.
	_, err = env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-U2", Name: "Dup link", UserId: env.ownerID, BaseSalary: 1000,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	require.Equal(t, "payroll.user_taken", tokenOf(t, err))

	// A non-existent user id is rejected.
	_, err = env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-U3", Name: "Bad link", UserId: "00000000-0000-0000-0000-0000000000ff",
	}))
	require.Error(t, err)
	require.Equal(t, "payroll.user_not_found", tokenOf(t, err))
}
