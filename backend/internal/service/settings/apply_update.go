package settings

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/update"
)

// ApplyUpdate downloads + verifies + stages the latest release next to the
// running binary (justmart.exe.new); the portable launcher swaps it in on the
// next start. Windows + portable only, OWNER-only (proto). It does NOT restart
// the server — the swap happens on the next launch (the request's restart flag
// is reserved for a future auto-relaunch; ignored today).
func (s *SettingsService) ApplyUpdate(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.ApplyUpdateRequest],
) (*connect.Response[settingsifacev1.ApplyUpdateResponse], error) {
	if runtime.GOOS != "windows" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("self-apply is only supported on the Windows portable build"))
	}
	if s.updateCfg.Disabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("updates are disabled"))
	}

	info, err := update.Check(ctx, updateHTTPClient, s.updateAPIBase, s.updateCfg.Repo, s.currentVersion())
	if err != nil || !info.Checked {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("couldn't reach the update server"))
	}
	if !info.UpdateAvailable {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("already up to date"))
	}

	exePath, err := os.Executable()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	staged, err := update.Apply(ctx, updateHTTPClient, info, filepath.Dir(exePath))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	// Best-effort: the current version becomes justmart.exe.bak after the launcher
	// swap, so record it as the rollback target for the Updates "Revert" action.
	_ = common.SetUpdatePrevVersion(ctx, s.db, s.currentVersion())

	return connect.NewResponse(&settingsifacev1.ApplyUpdateResponse{
		StagedVersion: staged,
		Restarting:    false, // applied on next restart via the launcher swap
	}), nil
}
