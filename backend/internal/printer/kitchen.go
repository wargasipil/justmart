package printer

import (
	"fmt"
	"strings"
	"time"
)

// KitchenTicket is the rendering-ready view of ONE fire: the lines just sent to
// the kitchen, not the whole bill. Like Receipt, the caller denormalizes every
// name first — the printer package never touches the DB.
type KitchenTicket struct {
	TableCode  string // floor label ("T4"); empty for a counter order
	OrderType  string // DINE_IN | TAKEAWAY | DELIVERY | ""
	GuestCount int32
	Server     string    // who fired it, so the kitchen knows who to call
	FiredAt    time.Time
	Round      int       // 1 = first fire of this bill, 2 = second round, ...
	Items      []KitchenLine
}

type KitchenLine struct {
	Qty      int32
	UnitName string
	Name     string
	Note     string // "no ice", "extra pedas"; printed indented under the item
}

// RenderKitchenTicket produces an ESC/POS stream for a kitchen station.
//
// It is a deliberately different document from the customer receipt, not a
// variant of it: NO PRICES, NO TOTALS, NO SHOP HEADER. A cook needs to know what
// to make, for which table, in what order — money on the ticket is noise at
// best, and at worst it is what gets handed to the customer by mistake. The
// table code is printed double-size because it is the one thing read from across
// a hot kitchen.
func RenderKitchenTicket(t KitchenTicket, s Settings) []byte {
	s.normalize()
	b := NewBuilder()

	b.AlignCenter()
	b.Bold(true).Line(kitchenHeading(t)).Bold(false)
	if t.TableCode != "" {
		// The one line that has to be legible at a glance.
		b.DoubleSize().Line(t.TableCode).NormalSize()
	}
	b.Line(rule(s.Width, '='))

	b.AlignLeft()
	if !t.FiredAt.IsZero() {
		b.Line(t.FiredAt.Format("15:04:05"))
	}
	if t.Round > 1 {
		// A cook seeing "Round 2" knows the earlier items are already in hand.
		b.Bold(true).Line(fmt.Sprintf("PESANAN TAMBAHAN #%d", t.Round)).Bold(false)
	}
	if t.GuestCount > 0 {
		b.Line(fmt.Sprintf("Tamu: %d", t.GuestCount))
	}
	if t.Server != "" {
		b.Line("Pelayan: " + t.Server)
	}
	b.Line(rule(s.Width, '-'))

	for _, it := range t.Items {
		title := fmt.Sprintf("%d x %s", it.Qty, it.Name)
		if it.UnitName != "" {
			title = fmt.Sprintf("%d %s x %s", it.Qty, it.UnitName, it.Name)
		}
		b.Bold(true).Line(title).Bold(false)
		if note := strings.TrimSpace(it.Note); note != "" {
			// Indented so it reads as belonging to the item above, and starred so
			// it can't be mistaken for another dish.
			b.Line("   * " + note)
		}
	}
	b.Line(rule(s.Width, '='))

	b.Feed(4)
	b.Cut()
	// No drawer kick: a kitchen station has no cash drawer, and firing an order
	// is not a payment.
	return b.Bytes()
}

func kitchenHeading(t KitchenTicket) string {
	switch t.OrderType {
	case "TAKEAWAY":
		return "BUNGKUS"
	case "DELIVERY":
		return "ANTAR"
	default:
		return "DAPUR"
	}
}
