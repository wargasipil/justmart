package supplier

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
)

func (s *SupplierService) UpdateSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UpdateSupplierRequest],
) (*connect.Response[inventoryifacev1.UpdateSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"code":                strings.ToUpper(strings.TrimSpace(req.Msg.Code)),
		"name":                strings.TrimSpace(req.Msg.Name),
		"contact_email":       strings.TrimSpace(req.Msg.ContactEmail),
		"phone":               strings.TrimSpace(req.Msg.Phone),
		"address":             strings.TrimSpace(req.Msg.Address),
		"bank_name":           strings.TrimSpace(req.Msg.BankName),
		"bank_account_number": strings.TrimSpace(req.Msg.BankAccountNumber),
		"bank_account_holder": strings.TrimSpace(req.Msg.BankAccountHolder),
	}
	if updates["name"].(string) == "" || updates["code"].(string) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code and name required"))
	}
	if err := s.db.WithContext(ctx).Model(sup).Updates(updates).Error; err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("update supplier: %w", err))
	}
	return connect.NewResponse(&inventoryifacev1.UpdateSupplierResponse{Supplier: supplierToProto(sup)}), nil
}
