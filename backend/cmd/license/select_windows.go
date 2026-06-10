//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
)

// pickBusinessType falls back to a numbered stdin prompt on Windows, where
// gocliselect's arrow-key UI cannot build (its pkg/term dependency is broken on
// Windows). Same outcome: pick one BussinessType (excluding UNSPECIFIED).
func pickBusinessType() (settingsifacev1.BussinessType, error) {
	vals := businessTypeValues()
	// Prompt UI on stderr so stdout carries only the token (clean piping/redirect).
	fmt.Fprintln(os.Stderr, "Select business type:")
	for i, v := range vals {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, humanizeBusinessType(settingsifacev1.BussinessType_name[int32(v)]))
	}
	fmt.Fprint(os.Stderr, "Enter number: ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read selection: %w", err)
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(vals) {
		return 0, fmt.Errorf("invalid selection %q", strings.TrimSpace(line))
	}
	return settingsifacev1.BussinessType(vals[choice-1]), nil
}
