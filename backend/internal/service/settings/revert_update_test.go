package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

// RevertUpdate is rejected with FailedPrecondition both on non-Windows hosts
// (the platform gate, Linux CI) and on Windows when there's no justmart.exe.bak
// next to the test binary — either way it never reaches the version write, so a
// single unconditional assertion is OS-agnostic. Mirrors apply_update_test.go.
func TestRevertUpdate_NoBackupOrNonWindowsRejected(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(nil)
	svc.SetUpdate("1.2.0", config.Update{Repo: "owner/name"})

	_, err := svc.RevertUpdate(context.Background(), connect.NewRequest(&settingsifacev1.RevertUpdateRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// The rollback-target version label round-trips through app_settings.
func TestUpdatePrevVersion_RoundTrip(t *testing.T) {
	t.Parallel()
	db, _ := servicetest.New(t)
	ctx := context.Background()

	got, err := common.GetUpdatePrevVersion(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "", got) // unset by default

	require.NoError(t, common.SetUpdatePrevVersion(ctx, db, "1.2.0"))
	got, err = common.GetUpdatePrevVersion(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "1.2.0", got)
}
