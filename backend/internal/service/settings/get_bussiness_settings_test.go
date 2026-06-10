package settings_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	settingssvc "github.com/justmart/backend/internal/service/settings"
	"github.com/justmart/backend/internal/service/servicetest"
)

// GetBussinessSettings is a stub today: it accepts a BussinessType and returns
// an empty response without error.
func TestGetBussinessSettings_Stub(t *testing.T) {
	t.Parallel()
	svc := settingssvc.NewSettingsService(servicetest.NewDB(t, servicetest.NewConfig(t)))

	resp, err := svc.GetBussinessSettings(context.Background(), connect.NewRequest(&settingsifacev1.GetBussinessSettingsRequest{
		Type: settingsifacev1.BussinessType_BUSSINESS_TYPE_PHARMACY_SHOP,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
}
