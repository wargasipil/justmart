package grosirmigrate

import (
	"encoding/csv"
	"io"
	"strconv"
)

// bomUTF8 is the byte-order mark Excel needs to read a UTF-8 CSV as UTF-8.
// Spelled as hex escapes so the source file itself stays plain ASCII (a literal
// BOM anywhere but byte 0 is a Go compile error).
const bomUTF8 = "\xEF\xBB\xBF"

// csvHeader is the report's column order. Money columns are raw minor units (no
// thousands grouping) so a spreadsheet reads them as numbers; the grouped form
// is only for the terminal report.
var csvHeader = []string{
	"action",
	"reason",
	"product_id",
	"product_name",
	"unit_id",
	"unit_name",
	"unit_factor",
	"min_qty",
	"list_price",
	"tier_price",
	"discount_id",
	"discount_type",
	"discount_per_item",
	"discount_value",
	"discount_expires_at",
}

// WriteCSV serializes the whole plan — converted AND skipped rows, in plan order
// — so one file answers both "what changed" and "what didn't, and why".
//
// applied distinguishes a dry-run preview (action CONVERT) from the record of a
// run that actually wrote (action CONVERTED); a file archived after --apply must
// not read as a proposal. Skipped rows are SKIP either way, since nothing
// happened to them.
//
// Leads with a UTF-8 BOM, matching the frontend exporter (lib/csv.ts) — without
// it Excel mangles non-ASCII product names.
func WriteCSV(w io.Writer, plan *Plan, applied bool) error {
	if _, err := io.WriteString(w, bomUTF8); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, it := range plan.Items {
		action := "SKIP"
		if it.Convert {
			action = "CONVERT"
			if applied {
				action = "CONVERTED"
			}
		}
		rec := []string{
			action,
			it.Reason,
			it.ProductID,
			it.ProductName,
			it.UnitID,
			it.UnitName,
			strconv.FormatInt(it.UnitFactor, 10),
			strconv.FormatInt(int64(it.MinQty), 10),
			strconv.FormatInt(it.ListPrice, 10),
			strconv.FormatInt(it.TierPrice, 10),
			it.DiscountID,
			it.DiscountType,
			strconv.FormatBool(it.PerItem),
			strconv.FormatInt(it.DiscountValue, 10),
			it.ExpiresAt,
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
