package user

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/justmart/backend/internal/service/common"
)

// rolesByMode is the single source of truth for which roles may be assigned in
// each business mode (mirrored on the frontend in routes/Users.tsx). APOTEKER —
// the Rx-authority role — is pharmacy-only; OWNER/PHARMACIST/CASHIER are shared.
// There is no retail-only role, so retail is just the shared set.
var rolesByMode = map[int32]map[string]bool{
	common.BussinessTypePharmacyShop: {roleOwner: true, rolePharmacist: true, roleCashier: true, roleApoteker: true},
	common.BussinessTypeRetail:       {roleOwner: true, rolePharmacist: true, roleCashier: true},
	common.BussinessTypeUnspecified:  {roleOwner: true, rolePharmacist: true, roleCashier: true}, // unset == retail
}

// validateRoleForMode rejects assigning a role that doesn't belong to the shop's
// current business mode (e.g. APOTEKER in retail), keeping the role sets from
// mixing across modes. Called by CreateUser + UpdateUserRole — the only
// user-controllable role writers (EnsureBootstrapOwner only ever writes OWNER).
func (s *UserService) validateRoleForMode(ctx context.Context, roleStr string) error {
	mode, err := common.GetBussinessType(ctx, s.db)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	allowed := rolesByMode[mode]
	if allowed == nil {
		allowed = rolesByMode[common.BussinessTypeRetail] // unknown value → retail set
	}
	if !allowed[roleStr] {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("the "+roleStr+" role is not available in the current business mode"))
	}
	return nil
}
