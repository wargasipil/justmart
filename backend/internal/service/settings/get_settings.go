package settings

import (
	"context"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SettingsService) GetSettings(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetSettingsRequest],
) (*connect.Response[settingsifacev1.GetSettingsResponse], error) {
	threshold, err := common.GetLowStockThreshold(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	title, err := common.GetAppTitle(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bt, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&settingsifacev1.GetSettingsResponse{
		Settings: &settingsifacev1.Settings{
			LowStockThreshold: threshold,
			AppTitle:          title,
			BusinessType:      settingsifacev1.BussinessType(bt),
		},
	}), nil
}
