package printer

// White-box: wrapText is the width contract for a label's name line, and
// asserting it through RenderLabel's output would measure the interleaved
// ESC/POS control bytes as if they were text.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapText_NeverExceedsWidth(t *testing.T) {
	t.Parallel()
	cases := []string{
		"Paracetamol 500mg tablet salut selaput isi 10 strip per dus",
		"Amoxicillin",
		strings.Repeat("A", 40),
		"short " + strings.Repeat("B", 25) + " tail",
		"",
	}
	for _, in := range cases {
		for _, width := range []int{16, 32, 48} {
			for _, line := range wrapText(in, width) {
				require.LessOrEqual(t, len(line), width,
					"wrapText(%q, %d) produced an over-wide line %q", in, width, line)
			}
		}
	}
}

func TestWrapText_KeepsWordsWholeAndOrdered(t *testing.T) {
	t.Parallel()
	got := wrapText("Paracetamol 500mg tablet salut selaput", 32)
	require.Equal(t, []string{"Paracetamol 500mg tablet salut", "selaput"}, got)
}

func TestWrapText_HardBreaksAnUnbrokenWord(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"AAAAA", "AAAAA", "AA"}, wrapText(strings.Repeat("A", 12), 5))
}

// A long word arriving mid-line must flush what is already buffered first,
// rather than silently dropping it.
func TestWrapText_FlushesPendingLineBeforeHardBreak(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"ab", "CCCCC", "CC"}, wrapText("ab "+strings.Repeat("C", 7), 5))
}

func TestWrapText_AlwaysReturnsALine(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{""}, wrapText("", 32))
	require.Equal(t, []string{""}, wrapText("   ", 32))
	require.Equal(t, []string{"x"}, wrapText("x", 0), "non-positive width degrades to passthrough")
}
