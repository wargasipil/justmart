package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	backupifacev1 "github.com/justmart/backend/gen/backup_iface/v1"
)

// DeleteBackup removes one backup_<timestamp>/ directory. The name is
// validated against backupNameRe before any filesystem op, so `..` /
// absolute paths / slashes / unrelated dirs cannot be deleted via this RPC.
func (s *Backups) DeleteBackup(
	_ context.Context,
	req *connect.Request[backupifacev1.DeleteBackupRequest],
) (*connect.Response[backupifacev1.DeleteBackupResponse], error) {
	name := req.Msg.Name
	if !backupNameRe.MatchString(name) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("invalid backup name"))
	}
	dir := filepath.Join(s.directory, name)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("backup not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&backupifacev1.DeleteBackupResponse{}), nil
}
