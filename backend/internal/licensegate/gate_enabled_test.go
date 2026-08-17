//go:build license

package licensegate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Run with: go test -tags license ./internal/licensegate/...
// Everything here is offline — no licensing server is contacted.

func TestProductKey_IsJustmart(t *testing.T) {
	t.Parallel()

	// Every product in the getresolved catalog verifies against the same signing
	// key, so this comparison is the ONLY thing stopping a licence bought for
	// another product from unlocking Justmart. Changing it silently is how that
	// hole reopens.
	require.Equal(t, "justmart", ProductKey)
}

func TestCachePaths_LiveUnderCacheDir(t *testing.T) {
	t.Parallel()

	o := Options{CacheDir: filepath.Join("some", "dir")}.withDefaults()
	require.Equal(t, filepath.Join("some", "dir", "install.token"), installTokenPath(o))
	require.Equal(t, filepath.Join("some", "dir", "licence.token"), claimCachePath(o))
	require.NotEqual(t, installTokenPath(o), claimCachePath(o),
		"the durable install credential and the short-lived claim must not share a file")
}

func TestNewClient_FailsWithoutAnInstallToken(t *testing.T) {
	t.Parallel()

	o := Options{CacheDir: t.TempDir()}.withDefaults()

	_, err := newClient(o)
	require.Error(t, err, "no token file at all")

	require.NoError(t, os.WriteFile(installTokenPath(o), []byte("   \n"), 0o600))
	_, err = newClient(o)
	require.ErrorContains(t, err, "empty",
		"a blank token must fail loudly, not build a client that authenticates as nobody")
}

func TestNewClient_BuildsFromAPersistedToken(t *testing.T) {
	t.Parallel()

	o := Options{CacheDir: t.TempDir()}.withDefaults()
	require.NoError(t, os.WriteFile(installTokenPath(o), []byte("grli_test_token\n"), 0o600))

	c, err := newClient(o)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestSeedActivation_SkipsWhenAlreadyActivated(t *testing.T) {
	t.Parallel()

	o := Options{
		CacheDir:  t.TempDir(),
		LicenceID: "GR-TEST-TEST-TEST-TEST",
		// Unroutable on purpose: if the guard called out despite an existing
		// token, this test would hang or the file would be overwritten.
		BaseURL: "http://127.0.0.1:1",
	}.withDefaults()
	require.NoError(t, os.WriteFile(installTokenPath(o), []byte("grli_existing"), 0o600))

	seedActivation(context.Background(), o)

	got, err := os.ReadFile(installTokenPath(o))
	require.NoError(t, err)
	require.Equal(t, "grli_existing", string(got),
		"an already-activated machine must boot offline, not re-activate over the network")
}

func TestSeedActivation_UnreachableServerIsNotFatal(t *testing.T) {
	t.Parallel()

	o := Options{
		CacheDir:  t.TempDir(),
		LicenceID: "GR-TEST-TEST-TEST-TEST",
		BaseURL:   "http://127.0.0.1:1",
	}.withDefaults()

	// Best-effort by design: it must fall through to the interactive page, where
	// the operator can read the server's own message, rather than abort boot.
	require.NotPanics(t, func() { seedActivation(context.Background(), o) })
	_, err := os.Stat(installTokenPath(o))
	require.True(t, os.IsNotExist(err), "a failed activation must not leave a bogus token behind")
}
