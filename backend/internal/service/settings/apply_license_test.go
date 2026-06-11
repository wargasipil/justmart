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

func TestApplyLicense_HappyPath(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)

	token, err := license.Issue("Apotek Sehat", common.BussinessTypePharmacyShop, 0)
	require.NoError(t, err)

	resp, err := svc.ApplyLicense(context.Background(), connect.NewRequest(&settingsifacev1.ApplyLicenseRequest{Token: token}))
	require.NoError(t, err)
	require.Equal(t, "Apotek Sehat", resp.Msg.Name)
	require.Equal(t, settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP, resp.Msg.Type)

	// Side effects: business mode applied + token/name persisted for boot re-apply.
	bt, err := common.GetBussinessType(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, common.BussinessTypePharmacyShop, bt)
	stored, err := common.GetLicense(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, token, stored)
	name, err := common.GetLicenseName(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, "Apotek Sehat", name)
}

func TestApplyLicense_InvalidToken(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)

	_, err := svc.ApplyLicense(context.Background(), connect.NewRequest(&settingsifacev1.ApplyLicenseRequest{
		Token: "not-a-valid-jwt",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// Nothing persisted on failure.
	stored, err := common.GetLicense(context.Background(), db)
	require.NoError(t, err)
	require.Empty(t, stored)
}

func TestApplyLicense_EmptyToken(t *testing.T) {
	t.Parallel()
	db := servicetest.NewDB(t, servicetest.NewConfig(t))
	svc := settingssvc.NewSettingsService(db)

	_, err := svc.ApplyLicense(context.Background(), connect.NewRequest(&settingsifacev1.ApplyLicenseRequest{Token: "   "}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
