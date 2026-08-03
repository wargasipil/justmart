package grosirmigrate_test

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/justmart/backend/internal/grosirmigrate"
	"github.com/justmart/backend/internal/model"
)

// bomUTF8 mirrors the unexported constant in the package under test (this is a
// black-box test). Hex escapes keep the source ASCII — a literal BOM anywhere
// but byte 0 is a Go compile error.
const bomUTF8 = "\xEF\xBB\xBF"

// readCSV strips the Excel BOM and parses the report back into records.
func readCSV(t *testing.T, raw string) [][]string {
	t.Helper()
	require.True(t, strings.HasPrefix(raw, bomUTF8), "report must lead with a UTF-8 BOM for Excel")
	recs, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(raw, bomUTF8))).ReadAll()
	require.NoError(t, err)
	return recs
}

func col(t *testing.T, header, row []string, name string) string {
	t.Helper()
	for i, h := range header {
		if h == name {
			return row[i]
		}
	}
	t.Fatalf("column %q not found in %v", name, header)
	return ""
}

func TestWriteCSV_ReportsConvertedAndSkippedRows(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, baseID, _ := seedProduct(t, db, "SKU-CSV")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12,
	})
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 500, MinQty: 0,
	})

	got := plan(t, db, grosirmigrate.Options{})
	var buf bytes.Buffer
	require.NoError(t, grosirmigrate.WriteCSV(&buf, got, false))

	recs := readCSV(t, buf.String())
	require.Len(t, recs, 3) // header + both rows: a skip is reported, not dropped
	header := recs[0]

	// Rows follow plan order (product_id, min_qty, id), so min_qty 0 sorts first.
	skip, conv := recs[1], recs[2]
	require.Equal(t, "SKIP", col(t, header, skip, "action"))
	require.Contains(t, col(t, header, skip, "reason"), "not qty-gated")

	require.Equal(t, "CONVERT", col(t, header, conv, "action"))
	require.Empty(t, col(t, header, conv, "reason"))
	require.Equal(t, pid, col(t, header, conv, "product_id"))
	require.Equal(t, "SKU-CSV Item", col(t, header, conv, "product_name"))
	require.Equal(t, baseID, col(t, header, conv, "unit_id"))
	require.Equal(t, "pcs", col(t, header, conv, "unit_name"))
	require.Equal(t, "12", col(t, header, conv, "min_qty"))
	// Money stays raw minor units so a spreadsheet reads it as a number.
	require.Equal(t, "10000", col(t, header, conv, "list_price"))
	require.Equal(t, "9000", col(t, header, conv, "tier_price"))
	// The source rule is recorded because Apply deletes it.
	require.Equal(t, "PERCENT", col(t, header, conv, "discount_type"))
	require.Equal(t, "false", col(t, header, conv, "discount_per_item"))
	require.Equal(t, "1000", col(t, header, conv, "discount_value"))
	require.Empty(t, col(t, header, conv, "discount_expires_at"))
}

// A report archived after --apply must not read as a proposal.
func TestWriteCSV_AppliedMarksRowsConverted(t *testing.T) {
	t.Parallel()
	db := newDB(t)
	pid, _, _ := seedProduct(t, db, "SKU-CSV-APPLIED")
	seedDiscount(t, db, model.ProductDiscount{
		ProductID: pid, DiscountType: "PERCENT", Value: 1000, MinQty: 12,
	})

	got := plan(t, db, grosirmigrate.Options{})
	var buf bytes.Buffer
	require.NoError(t, grosirmigrate.WriteCSV(&buf, got, true))

	recs := readCSV(t, buf.String())
	require.Len(t, recs, 2)
	require.Equal(t, "CONVERTED", col(t, recs[0], recs[1], "action"))
}

func TestWriteCSV_EmptyPlanStillWritesHeader(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, grosirmigrate.WriteCSV(&buf, plan(t, newDB(t), grosirmigrate.Options{}), false))
	recs := readCSV(t, buf.String())
	require.Len(t, recs, 1)
	require.Equal(t, "action", recs[0][0])
}
