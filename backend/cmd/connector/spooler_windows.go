//go:build windows

package main

import "github.com/alexbrainman/printer"

// readPrinterNames lists the printers installed on this Windows host.
func readPrinterNames() ([]string, error) {
	return printer.ReadNames()
}

// printToSpooler sends raw ESC/POS bytes to a named Windows printer via the
// print spooler in RAW mode (no driver rendering). jobID names the spool doc.
func printToSpooler(name string, data []byte, jobID string) error {
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
