//go:build windows

// Package spooler prints raw bytes to a locally-installed printer via the OS
// print spooler in RAW mode (no driver rendering). It is the ONLY importer of
// the Windows-only github.com/alexbrainman/printer dependency; the
// //go:build !windows stub (spooler_other.go) keeps every other build target
// (cmd/server on Linux/Docker, the test suite, CI) from ever linking it.
//
// Used by both the print connector (cmd/connector) and usb print mode
// (SaleService.PrintReceipt when connector.mode == "usb").
package spooler

import "github.com/alexbrainman/printer"

// ReadNames lists the printers installed on this Windows host.
func ReadNames() ([]string, error) { return printer.ReadNames() }

// Print sends raw ESC/POS bytes to a named printer via the spooler in RAW mode.
// An empty name targets this host's default printer. jobID names the spool
// document (for the Windows print queue).
func Print(name string, data []byte, jobID string) error {
	if name == "" {
		def, err := printer.Default()
		if err != nil {
			return err
		}
		name = def
	}
	p, err := printer.Open(name)
	if err != nil {
		return err
	}
	defer p.Close()
	if err := p.StartDocument(jobID, "RAW"); err != nil {
		return err
	}
	defer p.EndDocument()
	if err := p.StartPage(); err != nil {
		return err
	}
	if _, err := p.Write(data); err != nil {
		return err
	}
	return p.EndPage()
}
