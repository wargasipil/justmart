package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/spooler"
)

// GetPrintingInfo reports the active print mode plus, in usb mode, the printers
// installed on the SERVER host (via the OS spooler; empty off-Windows or on
// error). Drives the mode-aware Settings ▸ Printing panel. Manager-tier read.
func (s *SettingsService) GetPrintingInfo(
	_ context.Context,
	_ *connect.Request[settingsifacev1.GetPrintingInfoRequest],
) (*connect.Response[settingsifacev1.GetPrintingInfoResponse], error) {
	mode := s.mode
	if mode == "" {
		mode = "tcp"
	}
	var printers []string
	if mode == "usb" {
		// Best-effort: a spooler error (or non-Windows host) just yields no
		// options — the panel falls back to "host default printer".
		if names, err := spooler.ReadNames(); err == nil {
			printers = names
		}
	}
	return connect.NewResponse(&settingsifacev1.GetPrintingInfoResponse{
		Mode:          mode,
		LocalPrinters: printers,
	}), nil
}
