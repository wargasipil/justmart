package payroll

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func lineByCode(lines []StatLine, code string) (StatLine, bool) {
	for _, l := range lines {
		if l.Code == code {
			return l, true
		}
	}
	return StatLine{}, false
}

func TestRoundBps(t *testing.T) {
	require.Equal(t, int64(50_000), roundBps(5_000_000, 100))  // 1%
	require.Equal(t, int64(25_000), roundBps(1_000_000, 250))  // 2.5%
	require.Equal(t, int64(0), roundBps(0, 100))               // zero base
	require.Equal(t, int64(0), roundBps(100, 0))               // zero rate
	require.Equal(t, int64(2), roundBps(1, 15_000))            // 1.5 -> half-up 2
	require.Equal(t, int64(13_500), roundBps(5_400_001, 25))   // boundary case
}

func TestTerRate_Boundaries(t *testing.T) {
	a := terCategoryA()
	require.Equal(t, int64(0), terRate(a, 5_400_000))   // exactly the 0% ceiling
	require.Equal(t, int64(25), terRate(a, 5_400_001))  // just over -> 0.25%
	require.Equal(t, int64(200), terRate(a, 10_000_000))
	require.Equal(t, int64(3400), terRate(a, 5_000_000_000)) // open top bracket
}

func TestComputeStatutory_OnlyBaseNoStatutory(t *testing.T) {
	// Nobody enrolled, base under the PPh21 0% ceiling -> no statutory lines.
	lines := ComputeStatutory(StatInput{
		Earnings: 5_000_000, TaxableBase: 5_000_000, BaseSalary: 5_000_000, PtkpStatus: "TK0",
	}, DefaultBPJSConfig(), DefaultPPh21Config())
	require.Empty(t, lines)
}

func TestComputeStatutory_AllEnrolled(t *testing.T) {
	in := StatInput{
		Earnings: 5_000_000, TaxableBase: 5_000_000, BaseSalary: 5_000_000, PtkpStatus: "TK0",
		Enrolled: BPJSEnrollment{Kes: true, Jht: true, Jp: true, Jkk: true, Jkm: true},
	}
	lines := ComputeStatutory(in, DefaultBPJSConfig(), DefaultPPh21Config())

	kesEmp, ok := lineByCode(lines, "BPJS_KES_EMP")
	require.True(t, ok)
	require.Equal(t, kindDeduction, kesEmp.Kind)
	require.Equal(t, int64(50_000), kesEmp.Amount) // 1% of 5M

	kesEr, ok := lineByCode(lines, "BPJS_KES_ER")
	require.True(t, ok)
	require.Equal(t, kindEmployerContrib, kesEr.Kind)
	require.Equal(t, int64(200_000), kesEr.Amount) // 4% of 5M

	jhtEmp, _ := lineByCode(lines, "BPJS_JHT_EMP")
	require.Equal(t, int64(100_000), jhtEmp.Amount) // 2%
	jpEmp, _ := lineByCode(lines, "BPJS_JP_EMP")
	require.Equal(t, int64(50_000), jpEmp.Amount) // 1%

	// JKK/JKM are employer-only (no employee deduction line).
	_, hasJkkEmp := lineByCode(lines, "BPJS_JKK_EMP")
	require.False(t, hasJkkEmp)
	jkkEr, _ := lineByCode(lines, "BPJS_JKK_ER")
	require.Equal(t, int64(12_000), jkkEr.Amount) // 0.24%

	// Under the 0% PPh21 ceiling -> no PPh21 line.
	_, hasPph := lineByCode(lines, "PPH21")
	require.False(t, hasPph)
}

func TestComputeStatutory_KesehatanCapBinds(t *testing.T) {
	in := StatInput{
		Earnings: 20_000_000, TaxableBase: 20_000_000, BaseSalary: 20_000_000, PtkpStatus: "TK0",
		Enrolled: BPJSEnrollment{Kes: true},
	}
	lines := ComputeStatutory(in, DefaultBPJSConfig(), DefaultPPh21Config())
	kesEmp, _ := lineByCode(lines, "BPJS_KES_EMP")
	require.Equal(t, int64(120_000), kesEmp.Amount) // 1% of the 12M cap, not 20M
}

func TestComputeStatutory_NotEnrolledNoLines(t *testing.T) {
	in := StatInput{
		Earnings: 10_000_000, TaxableBase: 10_000_000, BaseSalary: 10_000_000, PtkpStatus: "TK0",
		Enrolled: BPJSEnrollment{Kes: false},
	}
	lines := ComputeStatutory(in, DefaultBPJSConfig(), DefaultPPh21Config())
	_, hasKes := lineByCode(lines, "BPJS_KES_EMP")
	require.False(t, hasKes)
}

func TestComputeStatutory_PPh21(t *testing.T) {
	// Taxable 10M, TK0 (category A) -> 2% TER -> 200,000.
	in := StatInput{Earnings: 10_000_000, TaxableBase: 10_000_000, BaseSalary: 10_000_000, PtkpStatus: "TK0"}
	lines := ComputeStatutory(in, DefaultBPJSConfig(), DefaultPPh21Config())
	pph, ok := lineByCode(lines, "PPH21")
	require.True(t, ok)
	require.Equal(t, kindDeduction, pph.Kind)
	require.Equal(t, int64(200_000), pph.Amount)

	// K3 maps to category C (seeded == A) -> same rate at this base.
	in.PtkpStatus = "K3"
	lines = ComputeStatutory(in, DefaultBPJSConfig(), DefaultPPh21Config())
	pph, _ = lineByCode(lines, "PPH21")
	require.Equal(t, int64(200_000), pph.Amount)
}

func TestParseConfig_FallbackOnGarbage(t *testing.T) {
	require.Equal(t, DefaultBPJSConfig(), ParseBPJSConfig(""))
	require.Equal(t, DefaultBPJSConfig(), ParseBPJSConfig("not json"))
	require.Equal(t, DefaultPPh21Config().Method, ParsePPh21Config("").Method)
}
