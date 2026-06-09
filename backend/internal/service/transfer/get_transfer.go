package transfer

import (
	"context"

	"connectrpc.com/connect"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
)

func (s *TransferService) GetTransfer(
	ctx context.Context,
	req *connect.Request[warehouseifacev1.GetTransferRequest],
) (*connect.Response[warehouseifacev1.GetTransferResponse], error) {
	out, err := s.loadTransfer(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&warehouseifacev1.GetTransferResponse{Transfer: out}), nil
}
