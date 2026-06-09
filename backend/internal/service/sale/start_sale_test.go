package sale_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	salesvc "github.com/justmart/backend/internal/service/sale"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestStartSale_DefaultWarehouse(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	ownerID := servicetest.EnsureOwner(t, gormDB, cfg) // real users.id for the FK
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)

	// OWNER principal with no WarehouseID -> ResolveWarehouse falls back to the
	// migration-seeded default warehouse "MAIN". No extra seeding needed.
	ctx := servicetest.OwnerCtx(context.Background(), ownerID)

	resp, err := svc.StartSale(ctx, connect.NewRequest(&posifacev1.StartSaleRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Sale)
	require.NotEmpty(t, resp.Msg.Sale.Id)
	require.Equal(t, ownerID, resp.Msg.Sale.CashierUserId)
	require.Equal(t, posifacev1.SaleStatus_SALE_STATUS_DRAFT, resp.Msg.Sale.Status)
	require.NotEmpty(t, resp.Msg.Sale.WarehouseId) // resolved to MAIN
}

func TestStartSale_Unauthenticated(t *testing.T) {
	t.Parallel()
	gormDB, cfg := servicetest.New(t)
	svc := salesvc.NewSaleService(gormDB, cfg.Printer)

	// No principal in ctx -> auth.MustPrincipal returns CodeUnauthenticated.
	_, err := svc.StartSale(context.Background(), connect.NewRequest(&posifacev1.StartSaleRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
