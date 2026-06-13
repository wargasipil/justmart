package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/servicetest"
)

// Unset (no boot seed in tests) → empty header/footer.
func TestGetReceiptSettings_EmptyWhenUnset(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	resp, err := svc.GetReceiptSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetReceiptSettingsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Header)
	require.Empty(t, resp.Msg.Footer)
}

// SetReceiptSettings persists multi-line header/footer; GetReceiptSettings reads
// them back; ReceiptLines splits exactly as the printer renders them.
func TestSetReceiptSettings_RoundTrip(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()

	header := "TOKO JAYA\nJl. Merdeka 1\nBandung"
	footer := "Terima kasih"
	set, err := svc.SetReceiptSettings(ctx, connect.NewRequest(&settingsifacev1.SetReceiptSettingsRequest{
		Header: header, Footer: footer,
	}))
	require.NoError(t, err)
	require.Equal(t, header, set.Msg.Header)

	got, err := svc.GetReceiptSettings(ctx, connect.NewRequest(&settingsifacev1.GetReceiptSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, header, got.Msg.Header)
	require.Equal(t, footer, got.Msg.Footer)

	// The stored text splits into the receipt lines the printer renders.
	require.Equal(t, []string{"TOKO JAYA", "Jl. Merdeka 1", "Bandung"}, common.ReceiptLines(got.Msg.Header))
	require.Equal(t, []string{"Terima kasih"}, common.ReceiptLines(got.Msg.Footer))
}

// SeedReceiptDefaults populates from config defaults only when unset, and never
// overwrites an existing value (incl. a user-cleared empty one).
func TestSeedReceiptDefaults_SetIfAbsentOnly(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	ctx := context.Background()

	// First seed writes the config defaults.
	require.NoError(t, common.SeedReceiptDefaults(ctx, db, []string{"JUSTMART"}, []string{"Thank you!"}))
	h, f, err := common.GetReceiptText(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "JUSTMART", h)
	require.Equal(t, "Thank you!", f)

	// User customizes; a later seed (e.g. next boot) must NOT overwrite it.
	require.NoError(t, common.SetReceiptText(ctx, db, "MY SHOP", "Bye"))
	require.NoError(t, common.SeedReceiptDefaults(ctx, db, []string{"JUSTMART"}, []string{"Thank you!"}))
	h, f, err = common.GetReceiptText(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "MY SHOP", h)
	require.Equal(t, "Bye", f)
}
