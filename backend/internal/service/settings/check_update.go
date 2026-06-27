package settings

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/update"
)

// updateHTTPClient bounds the GitHub call so a slow/unreachable network can't
// hang the request.
var updateHTTPClient = &http.Client{Timeout: 12 * time.Second}

func (s *SettingsService) currentVersion() string {
	if s.version == "" {
		return "dev"
	}
	return s.version
}

// CheckUpdate reports the running version + the latest GitHub release. A remote
// failure (offline / 404 / rate-limited) is returned softly as checked=false —
// never a connect error — so the UI degrades gracefully. OWNER-only (proto).
func (s *SettingsService) CheckUpdate(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.CheckUpdateRequest],
) (*connect.Response[settingsifacev1.CheckUpdateResponse], error) {
	cur := s.currentVersion()

	// Rollback availability is local (a justmart.exe.bak next to the binary) — it
	// works offline and even when updates are disabled, so compute it up front and
	// report it on both branches. The version label is best-effort.
	canRevert := false
	backupVersion := ""
	if runtime.GOOS == "windows" {
		if exePath, err := os.Executable(); err == nil && update.HasBackup(filepath.Dir(exePath)) {
			canRevert = true
			backupVersion, _ = common.GetUpdatePrevVersion(ctx, s.db)
		}
	}

	if s.updateCfg.Disabled {
		return connect.NewResponse(&settingsifacev1.CheckUpdateResponse{
			CurrentVersion: cur,
			Enabled:        false,
			CanRevert:      canRevert,
			BackupVersion:  backupVersion,
		}), nil
	}

	info, _ := update.Check(ctx, updateHTTPClient, s.updateAPIBase, s.updateCfg.Repo, cur)
	canSelfApply := runtime.GOOS == "windows" && info.AssetURL != "" && info.ChecksumURL != ""
	return connect.NewResponse(&settingsifacev1.CheckUpdateResponse{
		CurrentVersion:  cur,
		Checked:         info.Checked,
		UpdateAvailable: info.UpdateAvailable,
		LatestVersion:   info.Latest,
		ReleaseNotes:    info.ReleaseNotes,
		ReleaseUrl:      info.ReleaseURL,
		PublishedAt:     info.PublishedAt,
		Enabled:         true,
		CanSelfApply:    canSelfApply,
		CanRevert:       canRevert,
		BackupVersion:   backupVersion,
	}), nil
}
