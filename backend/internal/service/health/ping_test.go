package health_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	healthifacev1 "github.com/justmart/backend/gen/health_iface/v1"
	healthsvc "github.com/justmart/backend/internal/service/health"
	"github.com/justmart/backend/internal/service/servicetest"
)

// Ping is a public RPC (no auth). It pings the DB and reports liveness. With a
// live, migrated SQLite handle both status and db come back "ok".
func TestPing_OK(t *testing.T) {
	t.Parallel()
	svc := healthsvc.NewHealthService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.Ping(context.Background(), connect.NewRequest(&healthifacev1.PingRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
	require.Equal(t, "ok", resp.Msg.Status)
	require.Equal(t, "ok", resp.Msg.Db)
}

// Error path: when the underlying sql.DB pool is closed, the handler does NOT
// return a transport error (Ping never errors); instead it degrades gracefully,
// keeping Status "ok" but reporting the DB ping failure in the Db field.
func TestPing_DBClosed(t *testing.T) {
	t.Parallel()
	gormDB := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := healthsvc.NewHealthService(gormDB)

	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close()) // kill the pool so PingContext fails.

	resp, err := svc.Ping(context.Background(), connect.NewRequest(&healthifacev1.PingRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
	require.Equal(t, "ok", resp.Msg.Status)
	require.NotEqual(t, "ok", resp.Msg.Db)
	require.Contains(t, resp.Msg.Db, "error:")
}
