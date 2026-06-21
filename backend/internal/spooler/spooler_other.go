//go:build !windows

package spooler

import "errors"

// ErrUnsupported is returned by Print on non-Windows builds — the OS print
// spooler dep is Windows-only, so usb / local printing requires the server (or
// connector) to run on Windows.
var ErrUnsupported = errors.New("local (USB) printing is only supported on Windows")

// ReadNames returns no printers off Windows.
func ReadNames() ([]string, error) { return nil, nil }

// Print is unsupported off Windows (keeps cmd/server + the test suite building
// on Linux/Docker/CI without ever linking the Windows spooler dependency).
func Print(_ string, _ []byte, _ string) error { return ErrUnsupported }
