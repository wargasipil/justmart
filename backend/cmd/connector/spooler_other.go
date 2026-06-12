//go:build !windows

package main

import "errors"

// Non-Windows stub so the package compiles on Linux/macOS CI + dev hosts. The
// real spooler (spooler_windows.go) is the only importer of the Windows-only
// alexbrainman/printer dep, keeping it out of every other build target.

func readPrinterNames() ([]string, error) {
	return nil, nil
}

func printToSpooler(_ string, _ []byte, _ string) error {
	return errors.New("printing is only supported on windows")
}
