package supplier

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SupplierService) UpdateSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateSupplierRequest],
) (*connect.Response[inventoryifacev1.UpdateSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	code := strings.ToUpper(strings.TrimSpace(req.Msg.Code))
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" || code == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "supplier.required")
	}
	db := s.db.WithContext(ctx)
	if taken, e := common.ExistsBy(db, &model.Supplier{}, "code = ? AND id <> ?", code, sup.ID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.code_taken")
	}
	if taken, e := common.ExistsBy(db, &model.Supplier{}, "name = ? AND active = ? AND id <> ?", name, true, sup.ID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.name_taken")
	}
	updates := map[string]any{
		"code":                code,
		"name":                name,
		"contact_email":       strings.TrimSpace(req.Msg.ContactEmail),
		"phone":               strings.TrimSpace(req.Msg.Phone),
		"address":             strings.TrimSpace(req.Msg.Address),
		"bank_name":           strings.TrimSpace(req.Msg.BankName),
		"bank_account_number": strings.TrimSpace(req.Msg.BankAccountNumber),
		"bank_account_holder": strings.TrimSpace(req.Msg.BankAccountHolder),
	}
	if err := db.Model(sup).Updates(updates).Error; err != nil {
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.code_taken")
	}
	return connect.NewResponse(&inventoryifacev1.UpdateSupplierResponse{Supplier: supplierToProto(sup)}), nil
}
