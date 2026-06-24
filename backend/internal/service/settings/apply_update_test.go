package settings_test

import (
	"context"
	"runtime"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/config"
	settingssvc "github.com/justmart/backend/internal/service/settings"
)

// ApplyUpdate refuses when updates are disabled (OS-independent).
func TestApplyUpdate_DisabledRejected(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(nil)
	svc.SetUpdate("1.2.0", config.Update{Disabled: true, Repo: "owner/name"})

	_, err := svc.ApplyUpdate(context.Background(), connect.NewRequest(&settingsifacev1.ApplyUpdateRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// On non-Windows hosts (CI/Linux), self-apply is gated off.
func TestApplyUpdate_NonWindowsRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows host: self-apply gate doesn't trip")
	}
	t.Parallel()
	svc := settingssvc.NewSettingsService(nil)
	svc.SetUpdate("1.2.0", config.Update{Repo: "owner/name"})

	_, err := svc.ApplyUpdate(context.Background(), connect.NewRequest(&settingsifacev1.ApplyUpdateRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
