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

// A fresh DB brands as the retail default (UNSPECIFIED + no title), so the login
// screen falls back to the built-in brand.
func TestGetBranding_Unconfigured(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetBranding(context.Background(), connect.NewRequest(&settingsifacev1.GetBrandingRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED, resp.Msg.BusinessType)
	require.Empty(t, resp.Msg.AppTitle)
}

// Once the owner configures pharmacy mode + a title in Settings ▸ General,
// GetBranding surfaces both so the login screen brands as that pharmacy.
func TestGetBranding_ConfiguredPharmacy(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()

	require.NoError(t, common.SetBussinessType(ctx, db, common.BussinessTypePharmacyShop))
	require.NoError(t, common.SetAppTitle(ctx, db, "Apotek Sehat"))

	resp, err := svc.GetBranding(ctx, connect.NewRequest(&settingsifacev1.GetBrandingRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, resp.Msg.BusinessType)
	require.Equal(t, "Apotek Sehat", resp.Msg.AppTitle)
}
