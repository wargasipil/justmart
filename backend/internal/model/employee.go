package model

import "time"

// Employee is a payroll staff record. It is separate from User (the login
// account): nullable UserID links an employee to a login when one exists, which
// is what enables sales-commission. Money fields are minor units.
type Employee struct {
	ID                string  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Code              string  `gorm:"uniqueIndex;not null"` // EMP-NNNN, editable
	Name              string  `gorm:"not null"`
	UserID            *string `gorm:"type:uuid;column:user_id"`
	Position          string  `gorm:"not null;default:''"`
	BaseSalary        int64   `gorm:"not null;default:0;column:base_salary"`
	CommissionPct     int32   `gorm:"not null;default:0;column:commission_pct"` // basis points of period revenue
	Npwp              string  `gorm:"not null;default:'';column:npwp"`
	PtkpStatus        string  `gorm:"not null;default:'TK0';column:ptkp_status"`
	BpjsKesNo         string  `gorm:"not null;default:'';column:bpjs_kes_no"`
	BpjsTkNo          string  `gorm:"not null;default:'';column:bpjs_tk_no"`
	BpjsKesEnrolled   bool    `gorm:"not null;default:false;column:bpjs_kes_enrolled"`
	BpjsJhtEnrolled   bool    `gorm:"not null;default:false;column:bpjs_jht_enrolled"`
	BpjsJpEnrolled    bool    `gorm:"not null;default:false;column:bpjs_jp_enrolled"`
	BpjsJkkEnrolled   bool    `gorm:"not null;default:false;column:bpjs_jkk_enrolled"`
	BpjsJkmEnrolled   bool    `gorm:"not null;default:false;column:bpjs_jkm_enrolled"`
	BankName          string  `gorm:"not null;default:'';column:bank_name"`
	BankAccountNumber string  `gorm:"not null;default:'';column:bank_account_number"`
	BankAccountHolder string  `gorm:"not null;default:'';column:bank_account_holder"`
	BankChannelCode   string  `gorm:"not null;default:'';column:bank_channel_code"` // payout channel (e.g. ID_BCA)
	JoinedAt          *time.Time `gorm:"type:date;column:joined_at"`
	Active            bool       `gorm:"not null;default:true"`
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Allowances []EmployeeAllowance `gorm:"foreignKey:EmployeeID"`
}

func (Employee) TableName() string { return "employees" }

// EmployeeAllowance is a recurring monthly tunjangan line on an employee. The
// taxable flag decides whether it enters the PPh21 taxable base when a payslip
// is generated.
type EmployeeAllowance struct {
	ID         string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	EmployeeID string `gorm:"not null;type:uuid;column:employee_id"`
	Label      string `gorm:"not null"`
	Amount     int64  `gorm:"not null;default:0"`
	Taxable    bool   `gorm:"not null;default:true"`
	Active     bool   `gorm:"not null;default:true"`
	CreatedAt  time.Time
}

func (EmployeeAllowance) TableName() string { return "employee_allowances" }
