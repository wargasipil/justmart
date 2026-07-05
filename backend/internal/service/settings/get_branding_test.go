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

// A fresh, unlicensed DB brands as the retail default (UNSPECIFIED + no name).
func TestGetBranding_Unlicensed(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetBranding(context.Background(), connect.NewRequest(&settingsifacev1.GetBrandingRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED, resp.Msg.BusinessType)
	require.Empty(t, resp.Msg.ShopName)
}

// Once a pharmacy license is applied (mode + holder name stored in app_settings),
// GetBranding surfaces both so the login screen can brand as the pharmacy.
func TestGetBranding_Pharmacy(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()

	require.NoError(t, common.SetBussinessType(ctx, db, common.BussinessTypePharmacyShop))
	require.NoError(t, common.SetLicense(ctx, db, "token-xyz", "Apotek Sehat"))

	resp, err := svc.GetBranding(ctx, connect.NewRequest(&settingsifacev1.GetBrandingRequest{}))
	require.NoError(t, err)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, resp.Msg.BusinessType)
	require.Equal(t, "Apotek Sehat", resp.Msg.ShopName)
}
