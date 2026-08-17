//go:build !license

// This file is compiled into EVERY build except the licensed flavor. It exists
// so cmd/server can call Verify/Watch unconditionally while the licensing SDK
// stays out of the dependency graph of the ordinary binaries — `go list -deps
// ./cmd/server` without the tag must not contain getresolved_license.

package licensegate

import "context"

// Enabled reports that this binary enforces no licence.
const Enabled = false

// ProductKey is empty in an unlicensed build. It is declared only so code that
// logs or reports the flavor compiles under both tags.
const ProductKey = ""

// Verify is a no-op: an unlicensed build is always allowed to serve.
func Verify(context.Context, Options) error { return nil }

// Watch is a no-op: there is nothing to re-check.
func Watch(context.Context, Options, func(string)) {}
