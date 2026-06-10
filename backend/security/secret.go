// Package security holds app-level secret material.
package security

// SecretRoot is the HMAC signing key for offline LICENSE tokens (see
// cmd/license). It is a fixed, baked-in 256-bit random secret so the license
// generator (cmd/license) and the app that verifies a license share the same
// key — a per-process random value could never verify a license signed by a
// separate run.
//
// SECURITY NOTE: this is a SYMMETRIC, in-binary secret — anyone with the binary
// can extract it and forge licenses. It's obfuscation-grade, not proof against
// a determined attacker. For real tamper resistance, move to asymmetric signing
// (sign with a private key held only by you; embed only the public key here).
//
// This is the LICENSE key only; the auth JWT uses cfg.Auth.JWTSecret, not this.
const SecretRoot = "639899a23e4ec2455aa7116ea1f063b07c142380fe5d8ea11cdea2c6dad4cc79"
