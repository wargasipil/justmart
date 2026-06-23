package model

import "time"

// PayrollRun is one monthly payroll cycle. Status flows
// DRAFT → APPROVED → PAID, or DRAFT/APPROVED → VOIDED. The rollup totals are
// recomputed from the run's payslips on every component edit.
type PayrollRun struct {
	ID              string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	RunNo           *string    `gorm:"uniqueIndex;column:run_no"` // PAY-YYYY-NNNN
	PeriodYear      int32      `gorm:"not null;column:period_year"`
	PeriodMonth     int32      `gorm:"not null;column:period_month"`
	Status          string     `gorm:"not null;default:'DRAFT'"`
	Note            string     `gorm:"not null;default:''"`
	TotalGross      int64      `gorm:"not null;default:0;column:total_gross"`
	TotalDeductions int64      `gorm:"not null;default:0;column:total_deductions"`
	TotalNet        int64      `gorm:"not null;default:0;column:total_net"`
	CreatedBy       string     `gorm:"not null;type:uuid;column:created_by"`
	ApprovedAt      *time.Time `gorm:"column:approved_at"`
	PaidAt          *time.Time `gorm:"column:paid_at"`
	VoidedAt        *time.Time `gorm:"column:voided_at"`
	CreatedAt       time.Time
	UpdatedAt       time.Time

	Payslips []Payslip `gorm:"foreignKey:RunID"`
}

func (PayrollRun) TableName() string { return "payroll_runs" }

// Payslip is one employee's pay for a run. Identity/comp fields are SNAPSHOTTED
// at generation so an approved/printed slip is immutable even if the employee
// record later changes. Gross/Net/Deductions are stored and recomputed from the
// components.
type Payslip struct {
	ID                string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	RunID             string `gorm:"not null;type:uuid;column:run_id"`
	EmployeeID        string `gorm:"not null;type:uuid;column:employee_id"`
	EmployeeCode      string `gorm:"not null;default:'';column:employee_code"`
	EmployeeName      string `gorm:"not null;default:'';column:employee_name"`
	Position          string `gorm:"not null;default:''"`
	Npwp              string `gorm:"not null;default:'';column:npwp"`
	PtkpStatus        string `gorm:"not null;default:'';column:ptkp_status"`
	BankName          string `gorm:"not null;default:'';column:bank_name"`
	BankAccountNumber string `gorm:"not null;default:'';column:bank_account_number"`
	BankAccountHolder string `gorm:"not null;default:'';column:bank_account_holder"`
	BankChannelCode   string `gorm:"not null;default:'';column:bank_channel_code"` // snapshot of employee channel
	BaseSalary        int64  `gorm:"not null;default:0;column:base_salary"`
	CommissionBase    int64  `gorm:"not null;default:0;column:commission_base"`
	Gross             int64  `gorm:"not null;default:0"`
	TotalDeductions   int64  `gorm:"not null;default:0;column:total_deductions"`
	Net               int64  `gorm:"not null;default:0"`
	EmployerCost      int64  `gorm:"not null;default:0;column:employer_cost"`
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Components []PayslipComponent `gorm:"foreignKey:PayslipID"`
}

func (Payslip) TableName() string { return "payslips" }

// PayslipComponent is one line of a payslip breakdown. Kind ∈
// EARNING/DEDUCTION/EMPLOYER_CONTRIB; Code identifies the source
// (BASE/ALLOWANCE/COMMISSION/BONUS/BPJS_*/PPH21/MANUAL). System-generated lines
// are seeded at run generation; the OWNER can edit (Overridden=true) or add
// MANUAL lines until the run is approved.
type PayslipComponent struct {
	ID              string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	PayslipID       string `gorm:"not null;type:uuid;column:payslip_id"`
	Kind            string `gorm:"not null"`
	Code            string `gorm:"not null"`
	Label           string `gorm:"not null;default:''"`
	Amount          int64  `gorm:"not null;default:0"`
	Taxable         bool   `gorm:"not null;default:false"`
	// No default tag: a `false` bool with a `default:true` tag is omitted from
	// the INSERT by GORM, so a manual (system_generated=false) line would wrongly
	// persist as true. Always send the explicit value.
	SystemGenerated bool `gorm:"not null;column:system_generated"`
	Overridden      bool `gorm:"not null;default:false"`
	SortOrder       int32  `gorm:"not null;default:0;column:sort_order"`
	CreatedAt       time.Time
}

func (PayslipComponent) TableName() string { return "payslip_components" }

// PayrollNoCounter is the per-year sequence behind PAY-YYYY-NNNN run numbers.
type PayrollNoCounter struct {
	Year    int `gorm:"primaryKey"`
	LastSeq int `gorm:"not null;default:0;column:last_seq"`
}

func (PayrollNoCounter) TableName() string { return "payroll_no_counters" }
