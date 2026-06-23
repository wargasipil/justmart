package supplier

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SupplierService) CreateSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateSupplierRequest],
) (*connect.Response[inventoryifacev1.CreateSupplierResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	if name == "" || code == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "supplier.required")
	}
	db := s.db.WithContext(ctx)
	// Pre-check uniqueness so we return a specific field token, not raw DB text.
	if taken, e := common.ExistsBy(db, &model.Supplier{}, "code = ?", code); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.code_taken")
	}
	if taken, e := common.ExistsBy(db, &model.Supplier{}, "name = ? AND active = ?", name, true); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.name_taken")
	}
	sup := model.Supplier{
		Code:              code,
		Name:              name,
		ContactEmail:      strings.TrimSpace(req.Msg.ContactEmail),
		Phone:             strings.TrimSpace(req.Msg.Phone),
		Address:           strings.TrimSpace(req.Msg.Address),
		BankName:          strings.TrimSpace(req.Msg.BankName),
		BankAccountNumber: strings.TrimSpace(req.Msg.BankAccountNumber),
		BankAccountHolder: strings.TrimSpace(req.Msg.BankAccountHolder),
		Active:            true,
	}
	if err := db.Create(&sup).Error; err != nil {
		// Backstop: a rare race lost the pre-check; never leak the raw constraint.
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.code_taken")
	}
	return connect.NewResponse(&inventoryifacev1.CreateSupplierResponse{Supplier: supplierToProto(&sup)}), nil
}
