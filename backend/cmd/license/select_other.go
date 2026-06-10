//go:build !windows

package main

import (
	"fmt"
	"strconv"

	"github.com/nexidian/gocliselect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
)

// pickBusinessType shows an interactive arrow-key menu of the BussinessType enum
// (excluding UNSPECIFIED) and returns the chosen value.
func pickBusinessType() (settingsifacev1.BussinessType, error) {
	menu := gocliselect.NewMenu("Select business type")
	for _, v := range businessTypeValues() {
		menu.AddItem(humanizeBusinessType(settingsifacev1.BussinessType_name[int32(v)]), strconv.Itoa(v))
	}
	id := menu.Display()
	if id == "" {
		return 0, fmt.Errorf("no business type selected")
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return 0, fmt.Errorf("invalid selection %q: %w", id, err)
	}
	return settingsifacev1.BussinessType(n), nil
}
