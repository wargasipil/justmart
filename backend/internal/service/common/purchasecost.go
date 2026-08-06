package common

// NetUnitCost is the per-BASE-unit purchase cost a receipt stamps on
// batches.cost_price — the single definition of "what this stock cost us".
//
// subtotal is the PO line's NET amount (gross minus its line discount) and
// orderedQty is in BASE units; gross is the per-base fallback for a line
// carrying no qty to divide by. ppnRate is the PO's PPN as a whole percent, or
// 0 when PPN is off.
//
// PPN is CAPITALIZED into the cost: it is not recoverable for a non-PKP shop,
// so it is part of what the goods cost rather than a tax to reclaim. A PKP
// shop crediting PPN masukan would be double-counting it here — see "PPN in
// purchasing" in CLAUDE.md before changing this.
//
// The PPN scale is applied BEFORE the single half-up rounding, so the result
// never picks up the drift a (round, then scale) order would introduce. At
// ppnRate 0 this is bit-for-bit the pre-PPN formula, so a PO without PPN
// stamps exactly the cost it always did.
//
// This is the ONLY place the derivation lives on the Go side: CreateReceipt
// (what a batch gets) and the products on-order valuation (what it will get)
// both call it, so "ongoing" and "ready" valuation can never sit on different
// bases. frontend/src/lib/purchaseLine.ts mirrors it for the on-screen
// preview — change one and you must change the other.
func NetUnitCost(subtotal, orderedQty, gross int64, ppnRate int32) int64 {
	if ppnRate < 0 {
		ppnRate = 0
	}
	if ppnRate > 100 {
		ppnRate = 100
	}
	scale := int64(100 + ppnRate)
	if orderedQty <= 0 {
		return (gross*scale + 50) / 100
	}
	return (subtotal*scale + orderedQty*50) / (orderedQty * 100)
}
