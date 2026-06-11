// Command license generates an offline Justmart license token (a JWT signed
// with the in-binary key from security/secret.go).
//
//  1. CLI built with github.com/urfave/cli/v3.
//  2. The license is a JWT signed (HS256) with security.SecretRoot.
//  3. It carries a holder name + a business type; the business type is the
//     BussinessType enum from settings_iface.v1, chosen via an interactive
//     arrow-key menu (github.com/nexidian/gocliselect).
//
// Usage:
//
//	license --name "Apotek Sehat" [--expires 365] [--out license.txt]
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/urfave/cli/v3"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/security"
)

func main() {
	cmd := &cli.Command{
		Name:  "license",
		Usage: "Generate a Justmart license token (JWT signed with the app's license key)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "license holder / business name",
				Required: true,
			},
			&cli.IntFlag{
				Name:    "expires",
				Aliases: []string{"e"},
				Usage:   "license validity in days (0 = perpetual)",
				Value:   0,
			},
			&cli.StringFlag{
				Name:    "out",
				Aliases: []string{"o"},
				Usage:   "write the token to this file instead of stdout",
			},
		},
		Action: run,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(_ context.Context, cmd *cli.Command) error {
	name := strings.TrimSpace(cmd.String("name"))
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}

	bt, err := pickBusinessType()
	if err != nil {
		return err
	}

	signed, err := buildLicenseToken(name, bt, cmd.Int("expires"))
	if err != nil {
		return fmt.Errorf("sign license: %w", err)
	}

	if out := strings.TrimSpace(cmd.String("out")); out != "" {
		if err := os.WriteFile(out, []byte(signed+"\n"), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		fmt.Printf("License for %q (%s) written to %s\n", name, humanizeBusinessType(settingsifacev1.BussinessType_name[int32(bt)]), out)
		return nil
	}
	fmt.Println(signed)
	return nil
}

// buildLicenseToken mints a license JWT (HS256, signed with security.SecretRoot)
// carrying the holder name + business type, and an optional expiry. Extracted
// from run() so it's unit-testable without the interactive picker.
func buildLicenseToken(name string, bt settingsifacev1.BussinessType, expiresDays int) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"name":             name,
		"business_type":    settingsifacev1.BussinessType_name[int32(bt)],
		"business_type_id": int32(bt),
		"iat":              now.Unix(),
	}
	if expiresDays > 0 {
		claims["exp"] = now.Add(time.Duration(expiresDays) * 24 * time.Hour).Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(security.SecretRoot))
}

// pickBusinessType is implemented per-platform: an interactive arrow-key menu
// (gocliselect) on Linux/macOS, a numbered stdin prompt on Windows (gocliselect's
// pkg/term dependency does not build on Windows). See select_other.go /
// select_windows.go.

// businessTypeValues returns the BussinessType enum values, sorted ascending and
// excluding UNSPECIFIED (0). Shared by both pickBusinessType implementations.
func businessTypeValues() []int {
	vals := make([]int, 0, len(settingsifacev1.BussinessType_name))
	for k := range settingsifacev1.BussinessType_name {
		if k != 0 {
			vals = append(vals, int(k))
		}
	}
	sort.Ints(vals)
	return vals
}

// humanizeBusinessType turns "BUSSINESS_TYPE_PHARMACY_SHOP" into "Pharmacy shop".
func humanizeBusinessType(name string) string {
	s := strings.TrimPrefix(name, "BUSSINESS_TYPE_")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return name
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
