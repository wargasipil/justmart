package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

// With nothing configured, GetBussinessSettings returns UNSPECIFIED + empty name.
func TestGetBussinessSettings_DefaultUnspecified(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetBussinessSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED, resp.Msg.Type)
	require.Empty(t, resp.Msg.Name)
}

// The licensed shop name is surfaced (all roles) for pharmacy-mode branding.
func TestGetBussinessSettings_ReturnsLicensedName(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()
	require.NoError(t, common.SetLicense(ctx, db, "stored-token", "Apotek Sehat"))

	resp, err := svc.GetBussinessSettings(ctx, connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{}))
	require.NoError(t, err)
	require.Equal(t, "Apotek Sehat", resp.Msg.Name)
}
