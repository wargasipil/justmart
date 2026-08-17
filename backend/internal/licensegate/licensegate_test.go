package licensegate

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The watch loop decides whether a running shop's till keeps working, so it is
// tested directly rather than only through the SDK call site that needs a
// licensing server to exercise.

func TestWatchLoop_ReportsFirstInvalidVerdictAndStops(t *testing.T) {
	t.Parallel()

	var checks atomic.Int32
	reasons := make(chan string, 4)

	watchLoop(context.Background(), time.Millisecond,
		func(context.Context) (bool, string) {
			if checks.Add(1) < 3 {
				return true, "ok"
			}
			return false, "expired"
		},
		func(reason string) { reasons <- reason },
	)

	// watchLoop returned on its own — no cancellation was needed.
	require.Equal(t, int32(3), checks.Load(), "should stop checking after the first invalid verdict")
	select {
	case got := <-reasons:
		require.Equal(t, "expired", got)
	default:
		t.Fatal("onInvalid was never called")
	}
	require.Empty(t, reasons, "onInvalid must fire exactly once")
}

func TestWatchLoop_ReturnsOnContextCancelWithoutFiring(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var fired atomic.Bool
	var checks atomic.Int32

	done := make(chan struct{})
	go func() {
		defer close(done)
		watchLoop(ctx, time.Millisecond,
			func(context.Context) (bool, string) { checks.Add(1); return true, "ok" },
			func(string) { fired.Store(true) },
		)
	}()

	require.Eventually(t, func() bool { return checks.Load() >= 3 }, 2*time.Second, time.Millisecond,
		"a valid licence must keep being re-checked, not checked once")
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not return after ctx was cancelled")
	}
	require.False(t, fired.Load(), "a normal shutdown must not look like a licence failure")
}

func TestWithDefaults_FillsEmptyFieldsOnly(t *testing.T) {
	t.Parallel()

	got := Options{}.withDefaults()
	require.Equal(t, DefaultBaseURL, got.BaseURL)
	require.Equal(t, DefaultCacheDir, got.CacheDir)
	require.Equal(t, DefaultGrace, got.Grace)
	require.Equal(t, DefaultRecheckInterval, got.RecheckInterval)
	// 0 means "wait until signalled" and must survive defaulting — a shop PC is
	// often switched on well before anyone types the licence id.
	require.Zero(t, got.ActivationTimeout)

	custom := Options{
		BaseURL:         "http://localhost:8080",
		CacheDir:        "/data/lic",
		Grace:           time.Hour,
		RecheckInterval: 5 * time.Minute,
	}.withDefaults()
	require.Equal(t, "http://localhost:8080", custom.BaseURL)
	require.Equal(t, "/data/lic", custom.CacheDir)
	require.Equal(t, time.Hour, custom.Grace)
	require.Equal(t, 5*time.Minute, custom.RecheckInterval)
}

func TestWithDefaults_NegativeIntervalFallsBackInsteadOfPanicking(t *testing.T) {
	t.Parallel()

	// time.NewTicker panics on a non-positive duration, which would take the
	// whole server down at boot over a typo in config.yaml.
	require.Equal(t, DefaultRecheckInterval, Options{RecheckInterval: -time.Second}.withDefaults().RecheckInterval)
}
