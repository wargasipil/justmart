//go:build !license

package licensegate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The whole point of the tag pair is that an ordinary build behaves as if none
// of this existed. If Verify ever blocked or Watch ever fired here, every
// unlicensed flavor (Docker, Fly, the plain portable zip, the installer) would
// stall at boot — so pin it.

func TestDisabledGate_VerifyPassesImmediately(t *testing.T) {
	t.Parallel()

	require.False(t, Enabled)

	done := make(chan error, 1)
	go func() { done <- Verify(context.Background(), Options{}) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Verify blocked in an unlicensed build")
	}
}

func TestDisabledGate_WatchReturnsWithoutFiring(t *testing.T) {
	t.Parallel()

	fired := false
	done := make(chan struct{})
	go func() {
		defer close(done)
		Watch(context.Background(), Options{RecheckInterval: time.Millisecond}, func(string) { fired = true })
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch blocked in an unlicensed build")
	}
	require.False(t, fired)
}
