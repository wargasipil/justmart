package printer

import "time"

// Payslip is the rendering-ready view of one payslip (slip gaji). The caller
// denormalizes everything; the printer package does not touch the DB. Employer
// contributions are intentionally NOT shown on the employee slip.
type Payslip struct {
	RunNo           string
	PeriodLabel     string // e.g. "Juni 2026"
	EmployeeName    string
	EmployeeCode    string
	Position        string
	Bank            string // e.g. "BCA 12345 (a.n. Budi)"; empty = omit
	Earnings        []PayslipLine
	Deductions      []PayslipLine
	Gross           int64
	TotalDeductions int64
	Net             int64
	GeneratedAt     time.Time
}

type PayslipLine struct {
	Label  string
	Amount int64
}

// RenderPayslip produces a complete ESC/POS byte stream for a payslip, reusing
// the receipt builder + layout helpers (rule/twoCol/formatIDR).
func RenderPayslip(p Payslip, s Settings) []byte {
	s.normalize()
	b := NewBuilder()

	// Shop header (same branding as receipts).
	b.AlignCenter()
	for i, line := range s.Header {
		if i == 0 {
			b.Bold(true).Line(line).Bold(false)
		} else {
			b.Line(line)
		}
	}
	if len(s.Header) > 0 {
		b.Line(rule(s.Width, '='))
	}
	b.Bold(true).Line("SLIP GAJI").Bold(false)

	// Meta block.
	b.AlignLeft()
	if p.RunNo != "" {
		b.Line("No: " + p.RunNo)
	}
	if p.PeriodLabel != "" {
		b.Line("Periode: " + p.PeriodLabel)
	}
	if !p.GeneratedAt.IsZero() {
		b.Line(p.GeneratedAt.Format("2006-01-02 15:04"))
	}
	b.Line(rule(s.Width, '-'))

	// Employee block.
	if p.EmployeeName != "" {
		b.Line("Nama   : " + p.EmployeeName)
	}
	if p.EmployeeCode != "" {
		b.Line("Kode   : " + p.EmployeeCode)
	}
	if p.Position != "" {
		b.Line("Jabatan: " + p.Position)
	}
	if p.Bank != "" {
		b.Line("Bank   : " + p.Bank)
	}
	b.Line(rule(s.Width, '-'))

	// Earnings.
	b.Bold(true).Line("PENDAPATAN").Bold(false)
	for _, l := range p.Earnings {
		b.Line(twoCol(s.Width, l.Label, formatIDR(l.Amount)))
	}
	b.Line(twoCol(s.Width, "Total pendapatan", formatIDR(p.Gross)))
	b.Line(rule(s.Width, '-'))

	// Deductions.
	b.Bold(true).Line("POTONGAN").Bold(false)
	if len(p.Deductions) == 0 {
		b.Line("-")
	}
	for _, l := range p.Deductions {
		b.Line(twoCol(s.Width, l.Label, formatIDR(l.Amount)))
	}
	b.Line(twoCol(s.Width, "Total potongan", formatIDR(p.TotalDeductions)))
	b.Line(rule(s.Width, '='))

	// Net.
	b.Bold(true).Line(twoCol(s.Width, "GAJI BERSIH", formatIDR(p.Net))).Bold(false)
	b.Line(rule(s.Width, '='))

	// Footer.
	b.AlignCenter()
	for _, line := range s.Footer {
		b.Line(line)
	}
	b.Feed(4)
	b.Cut()
	return b.Bytes()
}
