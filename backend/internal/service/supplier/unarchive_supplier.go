package supplier

import (
	"context"

	"connectrpc.com/connect"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *SupplierService) UnarchiveSupplier(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.UnarchiveSupplierRequest],
) (*connect.Response[inventoryifacev1.UnarchiveSupplierResponse], error) {
	sup, err := s.load(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	// Restoring could collide with the partial unique index on (name) WHERE
	// active — another active supplier may have taken this name while it was
	// archived. (The code index is global and unchanged on a flip.)
	if taken, e := common.ExistsBy(s.db.WithContext(ctx), &model.Supplier{},
		"name = ? AND active = ? AND id <> ?", sup.Name, true, sup.ID); e != nil {
		return nil, connect.NewError(connect.CodeInternal, e)
	} else if taken {
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.name_taken")
	}
	if err := s.db.WithContext(ctx).Model(sup).Update("active", true).Error; err != nil {
		// Backstop: a rare race lost the pre-check; never leak the raw constraint.
		return nil, common.TokenError(connect.CodeAlreadyExists, "supplier.name_taken")
	}
	sup.Active = true
	return connect.NewResponse(&inventoryifacev1.UnarchiveSupplierResponse{Supplier: supplierToProto(sup)}), nil
}
