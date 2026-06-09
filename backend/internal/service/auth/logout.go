package auth

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
)

func (s *AuthService) Logout(
	ctx context.Context,
	req *connect.Request[userifacev1.LogoutRequest],
) (*connect.Response[userifacev1.LogoutResponse], error) {
	// Best-effort: unknown / already-revoked tokens succeed silently.
	if err := s.refresh.Revoke(ctx, req.Msg.RefreshToken); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&userifacev1.LogoutResponse{}), nil
}
