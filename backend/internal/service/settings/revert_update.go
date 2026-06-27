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

// RevertUpdate stages the previous-version backup (justmart.exe.bak) as the
// update the portable launcher applies on next start — rolling back the last
// update. Local only (no network), so it's NOT gated on update.disabled; the
// launcher swap is the same path a forward update uses. Windows + portable only,
// OWNER-only (proto). Does NOT restart — the swap happens on the next launch.
func (s *SettingsService) RevertUpdate(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.RevertUpdateRequest],
) (*connect.Response[settingsifacev1.RevertUpdateResponse], error) {
	if runtime.GOOS != "windows" {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("revert is only supported on the Windows portable build"))
	}

	exePath, err := os.Executable()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := update.StageRevert(filepath.Dir(exePath)); err != nil {
		if errors.Is(err, update.ErrNoBackup) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("no previous version to revert to"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Best-effort: the version we're reverting FROM becomes the backup after the
	// launcher swap, so it labels the next rollback target. A write failure must
	// not fail the revert itself.
	_ = common.SetUpdatePrevVersion(ctx, s.db, s.currentVersion())

	return connect.NewResponse(&settingsifacev1.RevertUpdateResponse{Staged: true}), nil
}
