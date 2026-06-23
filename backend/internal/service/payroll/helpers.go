package payroll

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	payrollifacev1 "github.com/justmart/backend/gen/payroll_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// validPtkpStatuses is the accepted PTKP set; it drives the PPh21 TER category
// lookup. An unknown status would silently get a zero PPh21 rate, so creates /
// updates validate against this set.
var validPtkpStatuses = map[string]bool{
	"TK0": true, "TK1": true, "TK2": true, "TK3": true,
	"K0": true, "K1": true, "K2": true, "K3": true,
}

func (s *EmployeeService) load(ctx context.Context, id string) (*model.Employee, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var emp model.Employee
	err := s.db.WithContext(ctx).
		Preload("Allowances", func(db *gorm.DB) *gorm.DB { return db.Order("created_at") }).
		Where("id = ?", id).First(&emp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("employee %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &emp, nil
}

// employeeFields is the create/update payload shared by both mutations. The
// proto request types are distinct (lint), so each handler copies into this.
type employeeFields struct {
	code              string
	name              string
	userID            *string
	position          string
	baseSalary        int64
	commissionPct     int32
	npwp              string
	ptkpStatus        string
	bpjsKesNo         string
	bpjsTkNo          string
	bpjsKesEnrolled   bool
	bpjsJhtEnrolled   bool
	bpjsJpEnrolled    bool
	bpjsJkkEnrolled   bool
	bpjsJkmEnrolled   bool
	bankName          string
	bankAccountNumber string
	bankAccountHolder string
	bankChannelCode   string
	joinedAt          *time.Time
	allowances        []model.EmployeeAllowance
}

// parseEmployeeFields validates + normalizes the common create/update inputs.
func parseEmployeeFields(
	code, name, userID, position string,
	baseSalary int64, commissionPct int32,
	npwp, ptkpStatus, bpjsKesNo, bpjsTkNo string,
	kes, jht, jp, jkk, jkm bool,
	bankName, bankAccountNumber, bankAccountHolder, bankChannelCode, joinedAt string,
	allowances []*payrollifacev1.EmployeeAllowanceInput,
) (*employeeFields, error) {
	f := &employeeFields{
		code:              strings.ToUpper(strings.TrimSpace(code)),
		name:              strings.TrimSpace(name),
		position:          strings.TrimSpace(position),
		baseSalary:        baseSalary,
		commissionPct:     commissionPct,
		npwp:              strings.TrimSpace(npwp),
		ptkpStatus:        strings.ToUpper(strings.TrimSpace(ptkpStatus)),
		bpjsKesNo:         strings.TrimSpace(bpjsKesNo),
		bpjsTkNo:          strings.TrimSpace(bpjsTkNo),
		bpjsKesEnrolled:   kes,
		bpjsJhtEnrolled:   jht,
		bpjsJpEnrolled:    jp,
		bpjsJkkEnrolled:   jkk,
		bpjsJkmEnrolled:   jkm,
		bankName:          strings.TrimSpace(bankName),
		bankAccountNumber: strings.TrimSpace(bankAccountNumber),
		bankAccountHolder: strings.TrimSpace(bankAccountHolder),
		bankChannelCode:   strings.ToUpper(strings.TrimSpace(bankChannelCode)),
	}
	if f.code == "" || f.name == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.required")
	}
	if f.baseSalary < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.salary_negative")
	}
	if f.commissionPct < 0 {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.commission_negative")
	}
	if f.ptkpStatus == "" {
		f.ptkpStatus = "TK0"
	}
	if !validPtkpStatuses[f.ptkpStatus] {
		return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.ptkp_invalid")
	}
	if uid := strings.TrimSpace(userID); uid != "" {
		f.userID = &uid
	}
	joined, err := parseDateMaybe(joinedAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	f.joinedAt = joined
	for _, in := range allowances {
		label := strings.TrimSpace(in.Label)
		if label == "" {
			return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.allowance_label_required")
		}
		if in.Amount < 0 {
			return nil, common.TokenError(connect.CodeInvalidArgument, "payroll.allowance_negative")
		}
		f.allowances = append(f.allowances, model.EmployeeAllowance{
			Label:   label,
			Amount:  in.Amount,
			Taxable: in.Taxable,
			Active:  true,
		})
	}
	return f, nil
}

// checkUserLink validates an optional user_id link: the user must exist and not
// already be linked to a different employee (the partial unique index backstops
// the race; this returns a specific token).
func (s *EmployeeService) checkUserLink(db *gorm.DB, userID *string, selfID string) error {
	if userID == nil {
		return nil
	}
	if exists, err := common.ExistsBy(db, &model.User{}, "id = ?", *userID); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	} else if !exists {
		return common.TokenError(connect.CodeInvalidArgument, "payroll.user_not_found")
	}
	q := "user_id = ?"
	args := []any{*userID}
	if selfID != "" {
		q += " AND id <> ?"
		args = append(args, selfID)
	}
	if taken, err := common.ExistsBy(db, &model.Employee{}, q, args...); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	} else if taken {
		return common.TokenError(connect.CodeAlreadyExists, "payroll.user_taken")
	}
	return nil
}

func parseDateMaybe(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(common.DateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("joined_at must be YYYY-MM-DD: %w", err)
	}
	return &t, nil
}

func employeeToProto(e *model.Employee) *payrollifacev1.Employee {
	out := &payrollifacev1.Employee{
		Id:                e.ID,
		Code:              e.Code,
		Name:              e.Name,
		UserId:            common.Deref(e.UserID),
		Position:          e.Position,
		BaseSalary:        e.BaseSalary,
		CommissionPct:     e.CommissionPct,
		Npwp:              e.Npwp,
		PtkpStatus:        e.PtkpStatus,
		BpjsKesNo:         e.BpjsKesNo,
		BpjsTkNo:          e.BpjsTkNo,
		BpjsKesEnrolled:   e.BpjsKesEnrolled,
		BpjsJhtEnrolled:   e.BpjsJhtEnrolled,
		BpjsJpEnrolled:    e.BpjsJpEnrolled,
		BpjsJkkEnrolled:   e.BpjsJkkEnrolled,
		BpjsJkmEnrolled:   e.BpjsJkmEnrolled,
		BankName:          e.BankName,
		BankAccountNumber: e.BankAccountNumber,
		BankAccountHolder: e.BankAccountHolder,
		BankChannelCode:   e.BankChannelCode,
		Active:            e.Active,
		CreatedAt:         e.CreatedAt.Unix(),
	}
	if e.JoinedAt != nil {
		out.JoinedAt = e.JoinedAt.Format(common.DateLayout)
	}
	for i := range e.Allowances {
		a := &e.Allowances[i]
		out.Allowances = append(out.Allowances, &payrollifacev1.EmployeeAllowance{
			Id:      a.ID,
			Label:   a.Label,
			Amount:  a.Amount,
			Taxable: a.Taxable,
		})
	}
	return out
}
