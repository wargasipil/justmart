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

// Unset (no boot seed in tests) → empty header/footer, default width (32).
func TestGetReceiptSettings_EmptyWhenUnset(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))
	resp, err := svc.GetReceiptSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetReceiptSettingsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Header)
	require.Empty(t, resp.Msg.Footer)
	require.Equal(t, common.DefaultReceiptWidth, resp.Msg.Width) // 32 (58mm) default
}

// Width round-trips through Set/Get; an invalid (<=0) width coerces to default.
func TestSetReceiptSettings_Width(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()

	set, err := svc.SetReceiptSettings(ctx, connect.NewRequest(&settingsifacev1.SetReceiptSettingsRequest{
		Header: "X", Footer: "Y", Width: 48, // 80mm
	}))
	require.NoError(t, err)
	require.Equal(t, int32(48), set.Msg.Width)

	got, err := svc.GetReceiptSettings(ctx, connect.NewRequest(&settingsifacev1.GetReceiptSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(48), got.Msg.Width)

	// A bad width is coerced to the default, never stored as <= 0.
	bad, err := svc.SetReceiptSettings(ctx, connect.NewRequest(&settingsifacev1.SetReceiptSettingsRequest{Width: 0}))
	require.NoError(t, err)
	require.Equal(t, common.DefaultReceiptWidth, bad.Msg.Width)
}

// SeedReceiptWidth populates from config only when unset, never overwriting.
func TestSeedReceiptWidth_SetIfAbsentOnly(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	ctx := context.Background()

	require.NoError(t, common.SeedReceiptWidth(ctx, db, 48)) // config = 80mm
	w, err := common.GetReceiptWidth(ctx, db)
	require.NoError(t, err)
	require.Equal(t, int32(48), w)

	// User switches to 58mm; a later seed (next boot) must NOT overwrite it.
	require.NoError(t, common.SetReceiptWidth(ctx, db, 32))
	require.NoError(t, common.SeedReceiptWidth(ctx, db, 48))
	w, err = common.GetReceiptWidth(ctx, db)
	require.NoError(t, err)
	require.Equal(t, int32(32), w)
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
