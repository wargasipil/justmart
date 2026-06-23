package payroll

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *EmployeeService) CreateEmployee(
	ctx context.Context,
	req *connect.Request[payrollifacev1.CreateEmployeeRequest],
) (*connect.Response[payrollifacev1.CreateEmployeeResponse], error) {
	m := req.Msg
	f, err := parseEmployeeFields(
		m.Code, m.Name, m.UserId, m.Position, m.BaseSalary, m.CommissionPct,
		m.Npwp, m.PtkpStatus, m.BpjsKesNo, m.BpjsTkNo,
		m.BpjsKesEnrolled, m.BpjsJhtEnrolled, m.BpjsJpEnrolled, m.BpjsJkkEnrolled, m.BpjsJkmEnrolled,
		m.BankName, m.BankAccountNumber, m.BankAccountHolder, m.BankChannelCode, m.JoinedAt, m.Allowances,
	)
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	if taken, e := common.ExistsBy(db, &model.Employee{}, "code = ?", f.code); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "payroll.code_taken")
	}
	if e := s.checkUserLink(db, f.userID, ""); e != nil {
		return nil, e
	}

	emp := model.Employee{
		Code:              f.code,
		Name:              f.name,
		UserID:            f.userID,
		Position:          f.position,
		BaseSalary:        f.baseSalary,
		CommissionPct:     f.commissionPct,
		Npwp:              f.npwp,
		PtkpStatus:        f.ptkpStatus,
		BpjsKesNo:         f.bpjsKesNo,
		BpjsTkNo:          f.bpjsTkNo,
		BpjsKesEnrolled:   f.bpjsKesEnrolled,
		BpjsJhtEnrolled:   f.bpjsJhtEnrolled,
		BpjsJpEnrolled:    f.bpjsJpEnrolled,
		BpjsJkkEnrolled:   f.bpjsJkkEnrolled,
		BpjsJkmEnrolled:   f.bpjsJkmEnrolled,
		BankName:          f.bankName,
		BankAccountNumber: f.bankAccountNumber,
		BankAccountHolder: f.bankAccountHolder,
		BankChannelCode:   f.bankChannelCode,
		JoinedAt:          f.joinedAt,
		Active:            true,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&emp).Error; err != nil {
			return common.TokenError(connect.CodeAlreadyExists, "payroll.code_taken")
		}
		if len(f.allowances) > 0 {
			for i := range f.allowances {
				f.allowances[i].EmployeeID = emp.ID
			}
			if err := tx.Create(&f.allowances).Error; err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, common.AsConnectErr(err)
	}

	full, err := s.load(ctx, emp.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&payrollifacev1.CreateEmployeeResponse{Employee: employeeToProto(full)}), nil
}
