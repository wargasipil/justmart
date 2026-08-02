package sale

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/justmart/backend/internal/model"
)

const (
	// How often the sweeper runs, and how long a DRAFT sale may sit idle before
	// it's considered abandoned. In-process, single-node posture (like the rate
	// limiter); not configurable for now.
	draftSweepInterval = time.Hour
	draftMaxIdle       = 24 * time.Hour
)

// SweepStaleDrafts hard-deletes DRAFT sales (and their items) whose updated_at is
// older than maxIdle, returning the number of sales deleted. Exported so tests
// can drive it directly without the goroutine. Safe to delete: a DRAFT has no
// stock_movements and no sale_no, so nothing references it (see DiscardSale).
//
// TABLE-BOUND drafts are exempt. An abandoned POS cart is garbage nobody will
// miss, which is what makes deleting it safe; an open bill on a table is a
// seated customer's order that someone is expected to come back to. A long
// service, an overnight tab or a forgotten table would all cross a 24h idle
// threshold, and silently deleting the order would lose real money. Clearing a
// stale table is a floor decision (settle it, or discard it explicitly), not
// something a background timer should make.
func SweepStaleDrafts(ctx context.Context, db *gorm.DB, maxIdle time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxIdle)
	var deleted int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Model(&model.Sale{}).
			Where("status = ? AND updated_at < ? AND table_id IS NULL", saleStatusDraft, cutoff).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("sale_id IN ?", ids).Delete(&model.SaleItem{}).Error; err != nil {
			return err
		}
		res := tx.Where("id IN ?", ids).Delete(&model.Sale{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})
	return deleted, err
}

// StartDraftSweeper launches a background goroutine that periodically deletes
// abandoned DRAFT carts (idle > draftMaxIdle) the POS client missed (crashes,
// lost sessions). Runs for the process lifetime; no graceful shutdown.
//
// It sweeps once immediately, then on every tick. The initial sweep matters:
// time.Ticker only fires after a full draftSweepInterval, so a process that
// restarts more often than that — container deploys, Fly machine restarts —
// would otherwise never sweep at all and let abandoned DRAFTs accumulate.
func StartDraftSweeper(db *gorm.DB) {
	sweep := func() {
		n, err := SweepStaleDrafts(context.Background(), db, draftMaxIdle)
		if err != nil {
			slog.Error("draft sweeper failed", "error", err)
			return
		}
		if n > 0 {
			slog.Info("draft sweeper deleted abandoned carts", "count", n)
		}
	}
	go func() {
		sweep()
		ticker := time.NewTicker(draftSweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			sweep()
		}
	}()
}
