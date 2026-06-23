package payroll_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
)

func TestUpdateEmployee_AllowancesFullReplace(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	created, err := env.svc.CreateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.CreateEmployeeRequest{
		Code: "EMP-UPD", Name: "Before", BaseSalary: 1000,
		Allowances: []*payrollifacev1.EmployeeAllowanceInput{
			{Label: "Transport", Amount: 100, Taxable: true},
			{Label: "Makan", Amount: 200, Taxable: true},
		},
	}))
	require.NoError(t, err)
	require.Len(t, created.Msg.Employee.Allowances, 2)

	upd, err := env.svc.UpdateEmployee(env.ctx, connect.NewRequest(&payrollifacev1.UpdateEmployeeRequest{
		Id: created.Msg.Employee.Id, Code: "EMP-UPD", Name: "After", BaseSalary: 2000,
		Allowances: []*payrollifacev1.EmployeeAllowanceInput{
			{Label: "Posisi", Amount: 750, Taxable: false},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "After", upd.Msg.Employee.Name)
	require.Equal(t, int64(2000), upd.Msg.Employee.BaseSalary)
	require.Len(t, upd.Msg.Employee.Allowances, 1)
	require.Equal(t, "Posisi", upd.Msg.Employee.Allowances[0].Label)
}

func TestArchiveEmployee(t *testing.T) {
	t.Parallel()
	env := newPayEnv(t)
	e := createEmployee(t, env, "Archive me", 1000)
	resp, err := env.svc.ArchiveEmployee(env.ctx, connect.NewRequest(&payrollifacev1.ArchiveEmployeeRequest{Id: e.Id}))
	require.NoError(t, err)
	require.False(t, resp.Msg.Employee.Active)

	// Excluded from the default list, included with include_inactive.
	list, err := env.svc.ListEmployees(env.ctx, connect.NewRequest(&payrollifacev1.ListEmployeesRequest{}))
	require.NoError(t, err)
	for _, row := range list.Msg.Employees {
		require.NotEqual(t, e.Id, row.Id)
	}
	listAll, err := env.svc.ListEmployees(env.ctx, connect.NewRequest(&payrollifacev1.ListEmployeesRequest{IncludeInactive: true}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, int(listAll.Msg.Total), 1)
}
