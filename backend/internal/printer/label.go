package printer

import "strings"

// MaxLabelCopies bounds a single PrintProductLabel call. A label printer is
// slow and a typo in the copies box ("100" for "10") is otherwise unstoppable
// once the bytes are on the wire — there is no cancel path through the
// connector.
const MaxLabelCopies = 100

// Label is the rendering-ready view of one product barcode label. The caller
// denormalizes the unit name / price before handing it in (the printer package
// does NOT touch the DB).
type Label struct {
	Name     string // product name; wrapped to the paper width
	SKU      string // barcode payload AND the human-readable line under it
	UnitName string // selling unit the price belongs to (e.g. "strip")
	Price    int64  // that unit's sell price, minor currency units
}

// LabelSettings collects the paper/appearance knobs. Zero values are filled in
// by normalize so callers can pass an empty struct in tests.
type LabelSettings struct {
	Width         int  // characters per line (32 = 58mm, 48 = 80mm)
	Copies        int  // how many identical labels; <=0 means 1
	BarcodeHeight byte // bar height in dots
	BarcodeWidth  byte // narrow-module width
}

func (s *LabelSettings) normalize() {
	if s.Width <= 0 {
		s.Width = 32
	}
	if s.Copies <= 0 {
		s.Copies = 1
	}
	if s.Copies > MaxLabelCopies {
		s.Copies = MaxLabelCopies
	}
	if s.BarcodeHeight == 0 {
		// The firmware default (162 dots) is over 2cm and swamps a shelf label.
		s.BarcodeHeight = 70
	}
	if s.BarcodeWidth == 0 {
		s.BarcodeWidth = MinModuleWidth
	}
}

// Module-width bounds for GS w. Epson-compatible firmware accepts 2-6; above 4
// a label-sized barcode is pointlessly wide, so we never pick more.
const (
	MinModuleWidth byte = 2
	MaxModuleWidth byte = 4
)

// dotsPerChar is the Font A cell width on a 203dpi thermal head, which is what
// makes the configured character width (32 = 58mm, 48 = 80mm) convertible to
// printable dots.
const dotsPerChar = 12

// PaperDots converts a configured character width to printable dots.
func PaperDots(widthChars int) int {
	if widthChars <= 0 {
		widthChars = 32
	}
	return widthChars * dotsPerChar
}

// symbolModules is the module count of a CODE128 symbol carrying n characters:
// start(11) + 11n + check(11) + stop(13).
func symbolModules(n int) int { return 35 + 11*n }

// quietZoneModules is the 10-module margin CODE128 requires on each side.
const quietZoneModules = 20

// FitModuleWidth picks the widest bar width whose CODE128 symbol still fits
// `paperDots`, preferring one that also leaves the required quiet zone. A wider
// module scans more reliably, so this maximizes rather than minimizes.
//
// ok=false means the data cannot be printed scannably at any supported width —
// the caller MUST refuse. Printing anyway is the worst outcome available: the
// printer silently truncates the symbol, producing a label that looks correct
// and scans as nothing (or, worse, as a different code).
func FitModuleWidth(dataLen, paperDots int) (width byte, ok bool) {
	sym := symbolModules(dataLen)
	// Preferred: symbol + full quiet zone.
	for w := MaxModuleWidth; w >= MinModuleWidth; w-- {
		if int(w)*(sym+quietZoneModules) <= paperDots {
			return w, true
		}
	}
	// Acceptable: the symbol itself fits. The label is centered, so the physical
	// paper margin beyond the printable area supplies some quiet zone.
	for w := MaxModuleWidth; w >= MinModuleWidth; w-- {
		if int(w)*sym <= paperDots {
			return w, true
		}
	}
	return MinModuleWidth, false
}

// RenderLabel produces the ESC/POS byte stream for `s.Copies` identical labels:
// product name, a CODE128 barcode of the SKU with the SKU printed under it, and
// the chosen unit's price. Each copy is cut separately so they tear apart.
//
// The caller must have checked Code128Encodable(l.SKU) — RenderLabel assumes a
// printable-ASCII SKU and does not validate.
func RenderLabel(l Label, s LabelSettings) []byte {
	s.normalize()
	b := NewBuilder()

	for i := 0; i < s.Copies; i++ {
		b.AlignCenter()

		// Name, bold, wrapped. Long names are the norm ("Paracetamol 500mg
		// tablet"), so wrapping rather than truncating keeps the label readable.
		b.Bold(true)
		for _, line := range wrapText(l.Name, s.Width) {
			b.Line(line)
		}
		b.Bold(false)

		// Barcode with the SKU as its own human-readable line, so a scanner
		// failure still leaves the code typeable at the till.
		b.Feed(1)
		b.BarcodeHeight(s.BarcodeHeight)
		b.BarcodeWidth(s.BarcodeWidth)
		b.BarcodeHRI(HRIBelow)
		b.Code128(l.SKU)
		b.Feed(1)

		// Price, double-size: it is what a customer reads from a shelf.
		priceLine := formatIDR(l.Price)
		if l.UnitName != "" {
			priceLine += " / " + l.UnitName
		}
		b.DoubleSize().Bold(true).Line(priceLine).Bold(false).NormalSize()

		b.Feed(3)
		b.Cut()
	}
	// Leave the printer left-aligned so a following job (a receipt) is not
	// silently centered by our trailing state.
	b.AlignLeft()
	return b.Bytes()
}

// wrapText breaks s into lines of at most width characters, splitting on spaces
// and hard-breaking any single word longer than the line. Returns at least one
// (possibly empty) line so the caller always prints something.
func wrapText(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := ""
	for _, w := range words {
		// A word that cannot fit on its own line is chopped into width-sized
		// pieces rather than overflowing the paper.
		for len(w) > width {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			out = append(out, w[:width])
			w = w[width:]
		}
		switch {
		case cur == "":
			cur = w
		case len(cur)+1+len(w) <= width:
			cur += " " + w
		default:
			out = append(out, cur)
			cur = w
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
