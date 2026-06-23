package payroll

import (
	"encoding/json"
	"strings"
)

// This file is the payroll statutory calculator: BPJS contributions + PPh21
// (Indonesian income-tax withholding, TER monthly method). It is pure (no DB, no
// ctx) so it is trivially unit-testable. Rates/tables are NOT hardcoded business
// logic — they are passed in from editable, seeded app_settings JSON
// (payroll_bpjs_config / payroll_pph21_config), so a shop can correct them
// without a migration. ComputeStatutory runs ONCE at run generation to seed
// payslip components; afterward the stored components are the source of truth and
// the OWNER may override any amount before approving.
//
// ⚠ The DEFAULT rates/tables below are provisional and MUST be confirmed by the
// shop's accountant before real payroll. The category B/C TER brackets are seeded
// equal to category A pending verification — see DefaultPPh21Config.

// Component kind + code constants (mirror the payslip_components CHECK set).
const (
	kindEarning         = "EARNING"
	kindDeduction       = "DEDUCTION"
	kindEmployerContrib = "EMPLOYER_CONTRIB"

	codeBase       = "BASE"
	codeAllowance  = "ALLOWANCE"
	codeCommission = "COMMISSION"
	codeBonus      = "BONUS"
	codeManual     = "MANUAL"
	codePPh21      = "PPH21"
)

// ---------------------------------------------------------------------------
// Config structs (marshalled to/from app_settings JSON)
// ---------------------------------------------------------------------------

// BPJSProgram is one BPJS program's contribution config. Rates are basis points
// (100 = 1%); WageCap is in minor units (0 = no cap); Basis ∈ "base" | "gross".
type BPJSProgram struct {
	Enabled bool   `json:"enabled"`
	EmpBps  int64  `json:"emp_bps"`
	ErBps   int64  `json:"er_bps"`
	WageCap int64  `json:"wage_cap"`
	Basis   string `json:"basis"`
}

type BPJSConfig struct {
	Kesehatan BPJSProgram `json:"kesehatan"`
	JHT       BPJSProgram `json:"jht"`
	JP        BPJSProgram `json:"jp"`
	JKK       BPJSProgram `json:"jkk"`
	JKM       BPJSProgram `json:"jkm"`
}

// TERBracket is one PPh21 TER monthly bracket: a monthly-gross ceiling Upto (0 =
// open top bracket) and the effective rate Bps (basis points).
type TERBracket struct {
	Upto int64 `json:"upto"`
	Bps  int64 `json:"bps"`
}

type PPh21Config struct {
	Method            string                  `json:"method"` // "TER_MONTHLY"
	Ptkp              map[string]int64        `json:"ptkp"`
	TerCategoryByPtkp map[string]string       `json:"ter_category_by_ptkp"`
	Ter               map[string][]TERBracket `json:"ter"`
}

// ---------------------------------------------------------------------------
// Defaults (provisional — flagged for accountant confirmation)
// ---------------------------------------------------------------------------

func DefaultBPJSConfig() BPJSConfig {
	return BPJSConfig{
		// Kesehatan: employee 1%, employer 4%, capped at Rp 12,000,000.
		Kesehatan: BPJSProgram{Enabled: true, EmpBps: 100, ErBps: 400, WageCap: 12_000_000, Basis: "gross"},
		// JHT: employee 2%, employer 3.7%, no cap.
		JHT: BPJSProgram{Enabled: true, EmpBps: 200, ErBps: 370, WageCap: 0, Basis: "gross"},
		// JP: employee 1%, employer 2%, capped at Rp 10,042,300.
		JP: BPJSProgram{Enabled: true, EmpBps: 100, ErBps: 200, WageCap: 10_042_300, Basis: "gross"},
		// JKK (work-accident): employer only, lowest risk class 0.24%.
		JKK: BPJSProgram{Enabled: true, EmpBps: 0, ErBps: 24, WageCap: 0, Basis: "gross"},
		// JKM (death): employer only, 0.30%.
		JKM: BPJSProgram{Enabled: true, EmpBps: 0, ErBps: 30, WageCap: 0, Basis: "gross"},
	}
}

// terCategoryA is the PMK 168/2023 TER monthly table for category A (PTKP TK/0,
// TK/1, K/0). Bps = effective rate in basis points; Upto = monthly-gross ceiling
// in minor units (final entry Upto=0 is the open top bracket).
func terCategoryA() []TERBracket {
	return []TERBracket{
		{5_400_000, 0}, {5_650_000, 25}, {5_950_000, 50}, {6_300_000, 75},
		{6_750_000, 100}, {7_500_000, 125}, {8_550_000, 150}, {9_650_000, 175},
		{10_050_000, 200}, {10_350_000, 225}, {10_700_000, 250}, {11_050_000, 300},
		{11_600_000, 350}, {12_500_000, 400}, {13_750_000, 500}, {15_100_000, 600},
		{16_950_000, 700}, {19_750_000, 800}, {24_150_000, 900}, {26_450_000, 1000},
		{28_000_000, 1100}, {30_050_000, 1200}, {32_400_000, 1300}, {35_400_000, 1400},
		{39_100_000, 1500}, {43_850_000, 1600}, {47_800_000, 1700}, {51_400_000, 1800},
		{56_300_000, 1900}, {62_200_000, 2000}, {68_600_000, 2100}, {77_500_000, 2200},
		{89_000_000, 2300}, {103_000_000, 2400}, {125_000_000, 2500}, {157_000_000, 2600},
		{206_000_000, 2700}, {337_000_000, 2800}, {454_000_000, 2900}, {550_000_000, 3000},
		{695_000_000, 3100}, {910_000_000, 3200}, {1_400_000_000, 3300}, {0, 3400},
	}
}

func DefaultPPh21Config() PPh21Config {
	// Category mapping per PMK 168/2023:
	//   A: TK/0, TK/1, K/0   B: TK/2, TK/3, K/1, K/2   C: K/3
	a := terCategoryA()
	return PPh21Config{
		Method: "TER_MONTHLY",
		// Annual PTKP amounts (minor units). Used only if a shop later switches to
		// the non-TER progressive method; TER's PTKP is implicit in the category.
		Ptkp: map[string]int64{
			"TK0": 54_000_000, "TK1": 58_500_000, "TK2": 63_000_000, "TK3": 67_500_000,
			"K0": 58_500_000, "K1": 63_000_000, "K2": 67_500_000, "K3": 72_000_000,
		},
		TerCategoryByPtkp: map[string]string{
			"TK0": "A", "TK1": "A", "K0": "A",
			"TK2": "B", "TK3": "B", "K1": "B", "K2": "B",
			"K3": "C",
		},
		// B and C are seeded equal to A as a working placeholder; replace with the
		// official category-B/C brackets after accountant review.
		Ter: map[string][]TERBracket{"A": a, "B": a, "C": a},
	}
}

func DefaultBPJSConfigJSON() string  { return mustJSON(DefaultBPJSConfig()) }
func DefaultPPh21ConfigJSON() string { return mustJSON(DefaultPPh21Config()) }

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParseBPJSConfig parses stored JSON, falling back to the default on empty/error.
func ParseBPJSConfig(s string) BPJSConfig {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultBPJSConfig()
	}
	var c BPJSConfig
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return DefaultBPJSConfig()
	}
	return c
}

// ParsePPh21Config parses stored JSON, falling back to the default on empty/error.
func ParsePPh21Config(s string) PPh21Config {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultPPh21Config()
	}
	var c PPh21Config
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return DefaultPPh21Config()
	}
	return c
}

// ---------------------------------------------------------------------------
// Calculator
// ---------------------------------------------------------------------------

// BPJSEnrollment captures which BPJS programs an employee participates in.
type BPJSEnrollment struct{ Kes, Jht, Jp, Jkk, Jkm bool }

// StatInput is everything the calculator needs, assembled by the caller from the
// employee + computed earnings.
type StatInput struct {
	Earnings    int64 // total gross (BASE + allowances + commission + bonus)
	TaxableBase int64 // sum of taxable EARNING lines
	BaseSalary  int64 // used when a program's basis == "base"
	PtkpStatus  string
	Enrolled    BPJSEnrollment
}

// StatLine is one suggested statutory component.
type StatLine struct {
	Kind    string
	Code    string
	Label   string
	Amount  int64
	Taxable bool
}

// ComputeStatutory returns the suggested BPJS employee-deduction +
// employer-contribution lines and the PPh21 deduction line. Every amount is a
// suggestion the OWNER may override per component.
func ComputeStatutory(in StatInput, bpjs BPJSConfig, tax PPh21Config) []StatLine {
	var lines []StatLine

	type prog struct {
		enrolled       bool
		cfg            BPJSProgram
		empCode, erCode, label string
	}
	progs := []prog{
		{in.Enrolled.Kes, bpjs.Kesehatan, "BPJS_KES_EMP", "BPJS_KES_ER", "BPJS Kesehatan"},
		{in.Enrolled.Jht, bpjs.JHT, "BPJS_JHT_EMP", "BPJS_JHT_ER", "BPJS JHT"},
		{in.Enrolled.Jp, bpjs.JP, "BPJS_JP_EMP", "BPJS_JP_ER", "BPJS JP"},
		{in.Enrolled.Jkk, bpjs.JKK, "", "BPJS_JKK_ER", "BPJS JKK"},
		{in.Enrolled.Jkm, bpjs.JKM, "", "BPJS_JKM_ER", "BPJS JKM"},
	}
	for _, p := range progs {
		if !p.enrolled || !p.cfg.Enabled {
			continue
		}
		wage := in.Earnings
		if p.cfg.Basis == "base" {
			wage = in.BaseSalary
		}
		capped := wage
		if p.cfg.WageCap > 0 && capped > p.cfg.WageCap {
			capped = p.cfg.WageCap
		}
		if p.empCode != "" {
			if emp := roundBps(capped, p.cfg.EmpBps); emp > 0 {
				lines = append(lines, StatLine{kindDeduction, p.empCode, p.label, emp, false})
			}
		}
		if er := roundBps(capped, p.cfg.ErBps); er > 0 {
			lines = append(lines, StatLine{kindEmployerContrib, p.erCode, p.label, er, false})
		}
	}

	if pph := computePPh21(in.TaxableBase, in.PtkpStatus, tax); pph > 0 {
		lines = append(lines, StatLine{kindDeduction, codePPh21, "PPh 21", pph, false})
	}
	return lines
}

func computePPh21(taxableBase int64, ptkp string, tax PPh21Config) int64 {
	cat := tax.TerCategoryByPtkp[strings.ToUpper(strings.TrimSpace(ptkp))]
	brackets := tax.Ter[cat]
	if len(brackets) == 0 {
		return 0
	}
	return roundBps(taxableBase, terRate(brackets, taxableBase))
}

// terRate returns the rate (bps) of the first bracket whose Upto >= base, or the
// open bracket (Upto == 0), which must be last.
func terRate(brackets []TERBracket, base int64) int64 {
	for _, b := range brackets {
		if b.Upto == 0 || base <= b.Upto {
			return b.Bps
		}
	}
	return brackets[len(brackets)-1].Bps
}

// roundBps applies a basis-points rate to a base amount, rounding half-up. Same
// convention as the discount/PPN money math elsewhere in the app.
func roundBps(base, bps int64) int64 {
	if base <= 0 || bps <= 0 {
		return 0
	}
	return (base*bps + 5000) / 10000
}
