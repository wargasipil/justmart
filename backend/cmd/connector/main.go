// Command connector is the Justmart print connector: a small program that runs
// next to the physical (USB/network) printer, dials the Justmart server over a
// long-lived stream, registers its locally-installed printers, and prints the
// receipts the server pushes — via the Windows print spooler. It is built only
// for GOOS=windows (the spooler dep is isolated behind //go:build windows; the
// non-windows stub exists only so the package still compiles on CI/dev hosts).
package main

import (
	"log/slog"
	"os"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	id, err := loadOrCreateIdentity()
	if err != nil {
		slog.Error("load identity", "error", err)
		os.Exit(1)
	}
	slog.Info("justmart connector starting",
		"server_url", cfg.ServerURL, "device", id.Name, "device_id", id.ID)
	// run loops forever (reconnecting); it only returns on an unrecoverable error.
	if err := run(cfg, id); err != nil {
		slog.Error("connector stopped", "error", err)
		os.Exit(1)
	}
}
