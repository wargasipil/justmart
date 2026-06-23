package payroll

import (
	"context"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *EmployeeService) UpdateEmployee(
	ctx context.Context,
	req *connect.Request[payrollifacev1.UpdateEmployeeRequest],
) (*connect.Response[payrollifacev1.UpdateEmployeeResponse], error) {
	emp, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
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
	if taken, e := common.ExistsBy(db, &model.Employee{}, "code = ? AND id <> ?", f.code, emp.ID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "payroll.code_taken")
	}
	if e := s.checkUserLink(db, f.userID, emp.ID); e != nil {
		return nil, e
	}

	updates := map[string]any{
		"code":                f.code,
		"name":                f.name,
		"user_id":             f.userID,
		"position":            f.position,
		"base_salary":         f.baseSalary,
		"commission_pct":      f.commissionPct,
		"npwp":                f.npwp,
		"ptkp_status":         f.ptkpStatus,
		"bpjs_kes_no":         f.bpjsKesNo,
		"bpjs_tk_no":          f.bpjsTkNo,
		"bpjs_kes_enrolled":   f.bpjsKesEnrolled,
		"bpjs_jht_enrolled":   f.bpjsJhtEnrolled,
		"bpjs_jp_enrolled":    f.bpjsJpEnrolled,
		"bpjs_jkk_enrolled":   f.bpjsJkkEnrolled,
		"bpjs_jkm_enrolled":   f.bpjsJkmEnrolled,
		"bank_name":           f.bankName,
		"bank_account_number": f.bankAccountNumber,
		"bank_account_holder": f.bankAccountHolder,
		"bank_channel_code":   f.bankChannelCode,
		"joined_at":           f.joinedAt,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Employee{}).Where("id = ?", emp.ID).Updates(updates).Error; err != nil {
			return common.TokenError(connect.CodeAlreadyExists, "payroll.code_taken")
		}
		// Full-replace the recurring allowance lines.
		if err := tx.Where("employee_id = ?", emp.ID).Delete(&model.EmployeeAllowance{}).Error; err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if len(f.allowances) > 0 {
			for i := range f.allowances {
				f.allowances[i].ID = ""
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
	return connect.NewResponse(&payrollifacev1.UpdateEmployeeResponse{Employee: employeeToProto(full)}), nil
}
