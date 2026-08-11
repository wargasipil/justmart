// Package printer renders POS receipts as ESC/POS byte streams and dispatches
// them to a network thermal printer over raw TCP.
//
// Only the subset of ESC/POS commands needed for a basic receipt is wired up;
// most network thermal printers (Epson, Star, generic Chinese 58/80mm) accept
// this dialect.
package printer

import "bytes"

// ESC/POS control bytes.
const (
	esc = 0x1B
	gs  = 0x1D
	lf  = 0x0A
)

// Builder accumulates ESC/POS bytes with convenience helpers.
type Builder struct{ buf bytes.Buffer }

func NewBuilder() *Builder {
	b := &Builder{}
	b.Init()
	return b
}

func (b *Builder) Bytes() []byte { return b.buf.Bytes() }

// Init resets the printer (clears formatting, line spacing, etc.).
func (b *Builder) Init() *Builder {
	b.buf.WriteByte(esc)
	b.buf.WriteByte('@')
	return b
}

// AlignLeft / AlignCenter / AlignRight set the justification for subsequent text.
func (b *Builder) AlignLeft() *Builder   { return b.align(0) }
func (b *Builder) AlignCenter() *Builder { return b.align(1) }
func (b *Builder) AlignRight() *Builder  { return b.align(2) }

func (b *Builder) align(n byte) *Builder {
	b.buf.WriteByte(esc)
	b.buf.WriteByte('a')
	b.buf.WriteByte(n)
	return b
}

// Bold toggles bold text.
func (b *Builder) Bold(on bool) *Builder {
	b.buf.WriteByte(esc)
	b.buf.WriteByte('E')
	if on {
		b.buf.WriteByte(1)
	} else {
		b.buf.WriteByte(0)
	}
	return b
}

// DoubleSize prints subsequent text at 2x width and height. Pair with
// NormalSize() to switch back.
func (b *Builder) DoubleSize() *Builder { return b.size(0x11) }
func (b *Builder) NormalSize() *Builder { return b.size(0x00) }

func (b *Builder) size(n byte) *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('!')
	b.buf.WriteByte(n)
	return b
}

// Text writes a raw string. No automatic line break.
func (b *Builder) Text(s string) *Builder {
	b.buf.WriteString(s)
	return b
}

// Line writes a string followed by a line feed.
func (b *Builder) Line(s string) *Builder {
	b.buf.WriteString(s)
	b.buf.WriteByte(lf)
	return b
}

// Feed advances `n` lines.
func (b *Builder) Feed(n int) *Builder {
	for i := 0; i < n; i++ {
		b.buf.WriteByte(lf)
	}
	return b
}

// Cut sends a partial-cut command. Many low-end printers also accept this as a
// no-op when they lack a cutter (their head just stays where it is).
func (b *Builder) Cut() *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('V')
	b.buf.WriteByte(1)
	return b
}

// HRI (human-readable interpretation) print positions for GS H.
const (
	HRINone  byte = 0
	HRIAbove byte = 1
	HRIBelow byte = 2
	HRIBoth  byte = 3
)

// BarcodeHeight sets the bar height in dots (GS h n). Typical label heights are
// 50-100; the printer default is 162, which eats a whole label.
func (b *Builder) BarcodeHeight(dots byte) *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('h')
	b.buf.WriteByte(dots)
	return b
}

// BarcodeWidth sets the narrow-module width (GS w n). Valid range is 2-6 on most
// firmware; 2 keeps a long SKU inside 58mm paper, 3 scans more reliably.
func (b *Builder) BarcodeWidth(n byte) *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('w')
	b.buf.WriteByte(n)
	return b
}

// BarcodeHRI sets where the human-readable digits print (GS H n) — use one of
// the HRI* constants.
func (b *Builder) BarcodeHRI(pos byte) *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('H')
	b.buf.WriteByte(pos)
	return b
}

// Code128 prints `data` as a CODE128 barcode using GS k function B
// (`GS k m n d1..dn`, m=73). The printer firmware does the symbology encoding —
// we only frame the payload — so there is no module table to get wrong here.
//
// The payload is prefixed with the code-set selector `{B`, which covers
// printable ASCII 32-126; a literal `{` in the data is escaped as `{{` per the
// CODE128 data convention. Callers must validate the data with
// Code128Encodable first: a byte outside code set B is silently mis-scanned by
// the printer rather than rejected, so this method drops nothing and checks
// nothing.
func (b *Builder) Code128(data string) *Builder {
	payload := []byte{'{', 'B'}
	for i := 0; i < len(data); i++ {
		if data[i] == '{' {
			payload = append(payload, '{')
		}
		payload = append(payload, data[i])
	}
	// n is a single byte: 255 max, and the {B prefix + escapes count toward it.
	if len(payload) > 255 {
		payload = payload[:255]
	}
	b.buf.WriteByte(gs)
	b.buf.WriteByte('k')
	b.buf.WriteByte(73) // m = 73: CODE128, length-prefixed form
	b.buf.WriteByte(byte(len(payload)))
	b.buf.Write(payload)
	return b
}

// Code128Encodable reports whether every byte of s is representable in CODE128
// code set B (printable ASCII, 0x20-0x7E). Empty strings are not encodable.
func Code128Encodable(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

// OpenDrawer fires the cash-drawer kick pulse on pin 2.
func (b *Builder) OpenDrawer() *Builder {
	// ESC p m t1 t2: m=0 (pin 2), t1=t2=50 (~100ms on, 100ms off).
	b.buf.WriteByte(esc)
	b.buf.WriteByte('p')
	b.buf.WriteByte(0)
	b.buf.WriteByte(50)
	b.buf.WriteByte(50)
	return b
}
