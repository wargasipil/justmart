package auth

import (
	"context"

	"connectrpc.com/connect"

	userifacev1 "github.com/justmart/backend/gen/user_iface/v1"
)

func (s *AuthService) Refresh(
	ctx context.Context,
	req *connect.Request[userifacev1.RefreshRequest],
) (*connect.Response[userifacev1.RefreshResponse], error) {
	newRaw, newExp, userID, role, err := s.refresh.Rotate(ctx, req.Msg.RefreshToken, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, err
	}

	accessTok, accessExp, err := s.access.Issue(userID, role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userifacev1.RefreshResponse{
		AccessToken:      accessTok,
		RefreshToken:     newRaw,
		AccessExpiresAt:  accessExp.Unix(),
		RefreshExpiresAt: newExp.Unix(),
	}), nil
}
