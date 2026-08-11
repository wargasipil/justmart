package printer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/printer"
)

func sampleLabel() printer.Label {
	return printer.Label{
		Name:     "Paracetamol 500mg",
		SKU:      "SKU-00123",
		UnitName: "strip",
		Price:    12500,
	}
}

// The GS k frame is the whole reason this feature scans: m must be 73 (CODE128),
// n must be the exact payload length, and the payload must start with the {B
// code-set selector. A wrong n silently truncates the symbol.
func TestRenderLabel_Code128FrameIsWellFormed(t *testing.T) {
	t.Parallel()
	out := printer.RenderLabel(sampleLabel(), printer.LabelSettings{})

	marker := []byte{0x1D, 'k', 73}
	i := bytes.Index(out, marker)
	require.GreaterOrEqual(t, i, 0, "GS k 73 (CODE128) command not emitted")

	n := int(out[i+3])
	payload := out[i+4 : i+4+n]
	require.Equal(t, "{BSKU-00123", string(payload))
	require.Len(t, payload, n, "declared length must match the payload written")
}

// A literal '{' is the one character CODE128 treats specially — it must be
// doubled, and n must count the escape.
func TestRenderLabel_Code128EscapesBrace(t *testing.T) {
	t.Parallel()
	l := sampleLabel()
	l.SKU = "A{B"
	out := printer.RenderLabel(l, printer.LabelSettings{})

	i := bytes.Index(out, []byte{0x1D, 'k', 73})
	require.GreaterOrEqual(t, i, 0)
	n := int(out[i+3])
	require.Equal(t, "{BA{{B", string(out[i+4:i+4+n]))
}

func TestRenderLabel_EmitsBarcodeGeometryAndHRI(t *testing.T) {
	t.Parallel()
	out := printer.RenderLabel(sampleLabel(), printer.LabelSettings{
		BarcodeHeight: 90,
		BarcodeWidth:  3,
	})
	require.Contains(t, string(out), string([]byte{0x1D, 'h', 90}), "GS h height")
	require.Contains(t, string(out), string([]byte{0x1D, 'w', 3}), "GS w module width")
	// HRI below: the SKU stays typeable when a scan fails.
	require.Contains(t, string(out), string([]byte{0x1D, 'H', 2}), "GS H HRI position")
}

func TestRenderLabel_ContainsNameAndPricedUnit(t *testing.T) {
	t.Parallel()
	out := string(printer.RenderLabel(sampleLabel(), printer.LabelSettings{}))
	require.Contains(t, out, "Paracetamol 500mg")
	require.Contains(t, out, "Rp 12.500 / strip")
}

func TestRenderLabel_OmitsUnitSuffixWhenUnnamed(t *testing.T) {
	t.Parallel()
	l := sampleLabel()
	l.UnitName = ""
	out := string(printer.RenderLabel(l, printer.LabelSettings{}))
	require.Contains(t, out, "Rp 12.500")
	require.NotContains(t, out, "/ ")
}

// Each copy is a full label ending in its own cut, so a strip of them tears apart.
func TestRenderLabel_CopiesRepeatAndEachCuts(t *testing.T) {
	t.Parallel()
	out := printer.RenderLabel(sampleLabel(), printer.LabelSettings{Copies: 3})
	require.Equal(t, 3, bytes.Count(out, []byte{0x1D, 'k', 73}), "one barcode per copy")
	require.Equal(t, 3, bytes.Count(out, []byte{0x1D, 'V', 1}), "one cut per copy")
	require.Equal(t, 3, strings.Count(string(out), "SKU-00123"))
}

func TestRenderLabel_CopiesDefaultToOneAndAreCapped(t *testing.T) {
	t.Parallel()
	one := printer.RenderLabel(sampleLabel(), printer.LabelSettings{Copies: 0})
	require.Equal(t, 1, bytes.Count(one, []byte{0x1D, 'k', 73}))

	capped := printer.RenderLabel(sampleLabel(), printer.LabelSettings{Copies: 5000})
	require.Equal(t, printer.MaxLabelCopies, bytes.Count(capped, []byte{0x1D, 'k', 73}))
}

// A long name must be broken up rather than overflowing the paper — otherwise
// the printer wraps it itself at an arbitrary column. (The width contract is
// pinned precisely on wrapText in label_wrap_test.go; this only checks the
// renderer actually routes the name through it.)
func TestRenderLabel_WrapsLongName(t *testing.T) {
	t.Parallel()
	l := sampleLabel()
	l.Name = "Paracetamol 500mg tablet salut selaput isi 10 strip per dus"
	out := string(printer.RenderLabel(l, printer.LabelSettings{Width: 32}))
	require.NotContains(t, out, l.Name, "name should have been split across lines")
	require.Contains(t, out, "Paracetamol 500mg tablet salut")
}

// The label leaves the printer left-aligned; otherwise the next receipt printed
// on the same device comes out centered.
func TestRenderLabel_ResetsAlignmentAtEnd(t *testing.T) {
	t.Parallel()
	out := printer.RenderLabel(sampleLabel(), printer.LabelSettings{})
	require.True(t, bytes.HasSuffix(out, []byte{0x1B, 'a', 0}), "should end with ESC a 0")
}

func TestCode128Encodable(t *testing.T) {
	t.Parallel()
	require.True(t, printer.Code128Encodable("SKU-00123"))
	require.True(t, printer.Code128Encodable("a~ !\"#{}"))
	require.False(t, printer.Code128Encodable(""), "empty has nothing to encode")
	require.False(t, printer.Code128Encodable("SKU\n1"), "control byte")
	require.False(t, printer.Code128Encodable("obat-é"), "non-ASCII")
}

// Bars are sized to the paper. A fixed module width overflows the print head on
// a long SKU, and the printer's response is to truncate the symbol — a label
// that looks right and scans as nothing.
func TestFitModuleWidth_PicksWidestThatFits(t *testing.T) {
	t.Parallel()
	dots58 := printer.PaperDots(32) // 384
	dots80 := printer.PaperDots(48) // 576
	require.Equal(t, 384, dots58)
	require.Equal(t, 576, dots80)

	// Short code on 58mm: room for a wider, more scannable bar.
	w, ok := printer.FitModuleWidth(6, dots58)
	require.True(t, ok)
	require.EqualValues(t, 3, w)

	// A 9-char SKU still fits at the minimum width, quiet zone included.
	w, ok = printer.FitModuleWidth(9, dots58)
	require.True(t, ok)
	require.EqualValues(t, 2, w)

	// 80mm has room for the same code at a wider module.
	w, ok = printer.FitModuleWidth(9, dots80)
	require.True(t, ok)
	require.EqualValues(t, 3, w)
}

// The regression this whole mechanism exists for: 16 chars needs 422 dots and
// 58mm prints 384, so it must be refused rather than silently truncated.
func TestFitModuleWidth_RefusesWhatCannotFit(t *testing.T) {
	t.Parallel()
	_, ok := printer.FitModuleWidth(16, printer.PaperDots(32))
	require.False(t, ok, "a 16-char SKU cannot print scannably on 58mm")

	// The same SKU is fine on 80mm paper.
	w, ok := printer.FitModuleWidth(16, printer.PaperDots(48))
	require.True(t, ok)
	require.EqualValues(t, 2, w)
}

// Whatever width is chosen, the symbol must physically fit the paper.
func TestFitModuleWidth_ChosenWidthAlwaysFits(t *testing.T) {
	t.Parallel()
	for _, chars := range []int{32, 48} {
		dots := printer.PaperDots(chars)
		for n := 1; n <= 40; n++ {
			w, ok := printer.FitModuleWidth(n, dots)
			if !ok {
				continue
			}
			require.LessOrEqual(t, int(w)*(35+11*n), dots,
				"width %d overflows %d dots for a %d-char code", w, dots, n)
			require.GreaterOrEqual(t, w, printer.MinModuleWidth)
			require.LessOrEqual(t, w, printer.MaxModuleWidth)
		}
	}
}
