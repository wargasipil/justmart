package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/license"
	"github.com/justmart/backend/internal/service/common"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

func TestGetLicenseInfo_None(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetLicenseInfo(context.Background(), connect.NewRequest(&settingsifacev1.GetLicenseInfoRequest{}))
	require.NoError(t, err)
	require.False(t, resp.Msg.HasLicense)
	require.Empty(t, resp.Msg.Name)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_UNSPECIFIED, resp.Msg.Type)
}

func TestGetLicenseInfo_AfterApply(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)

	token, err := license.Issue("Toko Retail", common.BussinessTypeRetail, 0)
	require.NoError(t, err)
	_, err = svc.ApplyLicense(context.Background(), connect.NewRequest(&settingsifacev1.ApplyLicenseRequest{Token: token}))
	require.NoError(t, err)

	resp, err := svc.GetLicenseInfo(context.Background(), connect.NewRequest(&settingsifacev1.GetLicenseInfoRequest{}))
	require.NoError(t, err)
	require.True(t, resp.Msg.HasLicense)
	require.Equal(t, "Toko Retail", resp.Msg.Name)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_RETAIL, resp.Msg.Type)
}

// The reported mode is derived from the stored LICENSE token, so it can't drift
// from the license even if the raw business_type setting is changed underneath.
func TestGetLicenseInfo_TypeFollowsLicenseNotStoredMode(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)
	ctx := context.Background()

	token, err := license.Issue("Apotek Sehat", common.BussinessTypePharmacyShop, 0)
	require.NoError(t, err)
	_, err = svc.ApplyLicense(ctx, connect.NewRequest(&settingsifacev1.ApplyLicenseRequest{Token: token}))
	require.NoError(t, err)

	// Drift the raw business_type out from under the license.
	require.NoError(t, common.SetBussinessType(ctx, db, common.BussinessTypeRetail))

	resp, err := svc.GetLicenseInfo(ctx, connect.NewRequest(&settingsifacev1.GetLicenseInfoRequest{}))
	require.NoError(t, err)
	// Still pharmacy — the license is the source of truth, not business_type.
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, resp.Msg.Type)
}
