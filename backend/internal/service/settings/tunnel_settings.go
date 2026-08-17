package settings

import (
	"context"
	"runtime"

	"connectrpc.com/connect"

	settingsifacev1 "github.com/justmart/backend/gen/settings_iface/v1"
	"github.com/justmart/backend/internal/service/common"
)

// GetTunnelSettings reports the Cloudflare Tunnel configuration: what is saved,
// what this process actually booted on, and which of the two the server is
// using. OWNER-only, and the raw token is never part of the answer.
func (s *SettingsService) GetTunnelSettings(
	ctx context.Context,
	_ *connect.Request[settingsifacev1.GetTunnelSettingsRequest],
) (*connect.Response[settingsifacev1.GetTunnelSettingsResponse], error) {
	state, err := s.tunnelState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	r := s.tunnelResponse(state)
	return connect.NewResponse(&settingsifacev1.GetTunnelSettingsResponse{
		Configured:      r.configured,
		TokenPreview:    r.preview,
		Active:          r.active,
		Source:          r.source,
		RestartRequired: r.restartRequired,
		Supported:       r.supported,
	}), nil
}

// SetTunnelSettings saves (or clears) the token. It does NOT start or stop
// cloudflared: the tunnel is launched once, next to the listener, from the
// token resolved at boot (App.Run). Restarting is therefore part of the
// instructions rather than a shortcoming to paper over — restart_required in
// the response is what the panel tells the owner.
func (s *SettingsService) SetTunnelSettings(
	ctx context.Context,
	req *connect.Request[settingsifacev1.SetTunnelSettingsRequest],
) (*connect.Response[settingsifacev1.SetTunnelSettingsResponse], error) {
	token, ok := common.NormalizeTunnelToken(req.Msg.Token)
	if !ok {
		return nil, common.TokenError(connect.CodeInvalidArgument, "settings.tunnel_token_invalid")
	}
	if err := common.SetCloudflareTunnelToken(ctx, s.db, token); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	state, err := s.tunnelState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	r := s.tunnelResponse(state)
	return connect.NewResponse(&settingsifacev1.SetTunnelSettingsResponse{
		Configured:      r.configured,
		TokenPreview:    r.preview,
		Active:          r.active,
		Source:          r.source,
		RestartRequired: r.restartRequired,
		Supported:       r.supported,
	}), nil
}

// tunnelState re-resolves the precedence with the CURRENT saved token, which is
// what makes "restart required" answerable: comparing it against the token this
// process booted on is the whole question.
func (s *SettingsService) tunnelState(ctx context.Context) (common.TunnelState, error) {
	return common.ResolveTunnel(ctx, s.db, s.tunnelCfgToken, s.tunnelEnvSet, s.tunnelEnvOff)
}

type tunnelView struct {
	configured      bool
	preview         string
	active          bool
	source          string
	restartRequired bool
	supported       bool
}

func (s *SettingsService) tunnelResponse(state common.TunnelState) tunnelView {
	return tunnelView{
		configured: state.Saved != "",
		preview:    common.MaskTunnelToken(state.Saved),
		// What the process is running, not what the settings say — those differ
		// exactly in the window this panel exists to explain.
		active: s.tunnelBootToken != "",
		source: string(state.Source),
		// The token that WOULD be used on the next boot vs the one in use now.
		restartRequired: state.Token != s.tunnelBootToken,
		supported:       runtime.GOOS == "windows",
	}
}
