package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/db"
	"github.com/justmart/backend/internal/grosirmigrate"
)

// discountToGrosirCmd is the `discount-to-grosir` subcommand: a one-way data
// migration turning qty-gated product discounts into grosir price tiers. It is a
// CLI command rather than a goose migration because it is a judgement call about
// a shop's pricing, not a schema change — it must be previewable, re-runnable,
// and skippable per row.
//
// It is DESTRUCTIVE (a converted discount is deleted), so it dry-runs by default
// and writes only with --apply.
var discountToGrosirCmd = &cli.Command{
	Name:  "discount-to-grosir",
	Usage: "Convert qty-gated product discounts into grosir price tiers (dry run unless --apply)",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to config.yaml (else $JUSTMART_CONFIG / ./config.yaml)",
		},
		&cli.BoolFlag{
			Name:  "apply",
			Usage: "actually write the tiers and delete the converted discounts",
		},
		&cli.BoolFlag{
			Name: "include-line-fixed",
			Usage: "also convert FIXED discounts that apply to the whole line " +
				"(amount is divided by the threshold: exact at min_qty, more generous above it)",
		},
		&cli.StringFlag{
			Name:  "csv",
			Usage: "write the full report (converted AND skipped) to this file; `-` for stdout",
		},
	},
	Action: runDiscountToGrosir,
}

func runDiscountToGrosir(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(cmd.String("config"))
	if err != nil {
		return err
	}
	gdb, err := db.Open(cfg)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		defer sqlDB.Close()
	}

	plan, err := grosirmigrate.BuildPlan(gdb.WithContext(ctx), grosirmigrate.Options{
		IncludeLineFixed: cmd.Bool("include-line-fixed"),
	})
	if err != nil {
		return err
	}
	csvPath := cmd.String("csv")
	// `--csv -` streams the report to stdout, so the human table would corrupt it.
	quiet := csvPath == "-"
	if !quiet {
		printPlan(plan)
	}

	convertible := plan.Convertible()
	applied := false
	switch {
	case len(convertible) == 0:
		if !quiet {
			fmt.Println("\nNothing to convert.")
		}
	case !cmd.Bool("apply"):
		if !quiet {
			fmt.Printf("\nDry run - nothing was written. Re-run with --apply to convert %d discount(s).\n", len(convertible))
		}
	default:
		n, err := grosirmigrate.Apply(gdb.WithContext(ctx), plan)
		if err != nil {
			// The CSV is deliberately NOT written here: the transaction rolled
			// back, so a report claiming CONVERTED would be a lie.
			return err
		}
		applied = true
		if !quiet {
			fmt.Printf("\nConverted %d discount(s) into grosir tiers.\n", n)
		}
	}

	// Written even when nothing converted — the skip reasons are the point of the
	// report as often as the conversions are.
	if csvPath != "" {
		if err := writePlanCSV(csvPath, plan, applied); err != nil {
			return err
		}
		if !quiet {
			fmt.Printf("Report written to %s (%d row(s)).\n", csvPath, len(plan.Items))
		}
	}
	return nil
}

func writePlanCSV(path string, plan *grosirmigrate.Plan, applied bool) error {
	if path == "-" {
		return grosirmigrate.WriteCSV(os.Stdout, plan, applied)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	if err := grosirmigrate.WriteCSV(f, plan, applied); err != nil {
		f.Close()
		return fmt.Errorf("write csv: %w", err)
	}
	return f.Close()
}

func printPlan(plan *grosirmigrate.Plan) {
	convertible := plan.Convertible()
	skipped := plan.Skipped()

	fmt.Printf("Scanned %d product discount(s): %d convertible, %d skipped.\n",
		len(plan.Items), len(convertible), len(skipped))

	if len(convertible) > 0 {
		fmt.Println("\nCONVERT")
		for _, it := range convertible {
			fmt.Printf("  %-28s %s >= %d  %s -> %s\n",
				truncate(it.ProductName, 28), it.UnitName, it.MinQty,
				money(it.ListPrice), money(it.TierPrice))
		}
	}
	if len(skipped) > 0 {
		fmt.Println("\nSKIP")
		for _, it := range skipped {
			fmt.Printf("  %-28s %s\n", truncate(it.ProductName, 28), it.Reason)
		}
	}
}

func truncate(s string, n int) string {
	if s == "" {
		return "(unnamed)"
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..." // ASCII ellipsis: a cmd console mangles U+2026
}

// money groups an integer minor-unit amount with dots (id-ID style) so the
// report is readable at a glance; the DB values themselves are untouched.
func money(v int64) string {
	s := strconv.FormatInt(v, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
