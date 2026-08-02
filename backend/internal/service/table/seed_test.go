package table_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	tableifacev1 "github.com/justmart/backend/gen/table_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/servicetest"
	tablesvc "github.com/justmart/backend/internal/service/table"
)

// newTableEnv is the common setup: fresh DB, bootstrap owner, table service, and
// an OWNER context (every RPC here resolves the caller's active warehouse).
func newTableEnv(t *testing.T) (*tablesvc.TableService, context.Context, *gorm.DB, string) {
	t.Helper()
	db, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, db, cfg)
	return tablesvc.NewTableService(db), servicetest.OwnerCtx(context.Background(), ownerID), db, ownerID
}

func createTable(
	t *testing.T,
	svc *tablesvc.TableService,
	ctx context.Context,
	code string,
) *tableifacev1.DiningTable {
	t.Helper()
	r, err := svc.CreateTable(ctx, connect.NewRequest(&tableifacev1.CreateTableRequest{
		Code: code, Name: code + " table", Area: "Indoor", Seats: 4,
	}))
	require.NoError(t, err)
	return r.Msg.Table
}

// defaultWarehouseID returns the migration-seeded default warehouse (MAIN) — the
// one an OWNER with no explicit membership resolves to.
func defaultWarehouseID(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var w model.Warehouse
	require.NoError(t, db.Where("is_default").First(&w).Error)
	return w.ID
}

func requireToken(t *testing.T, err error, code connect.Code, token string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, connect.CodeOf(err))
	var ce *connect.Error
	require.True(t, errors.As(err, &ce))
	require.Equal(t, token, ce.Message())
}
