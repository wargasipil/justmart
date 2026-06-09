package health

import (
	"context"

	"connectrpc.com/connect"

	healthifacev1 "github.com/justmart/backend/gen/health_iface/v1"
)

func (h *HealthService) Ping(
	ctx context.Context,
	_ *connect.Request[healthifacev1.PingRequest],
) (*connect.Response[healthifacev1.PingResponse], error) {
	dbStatus := "ok"
	sqlDB, err := h.db.DB()
	if err != nil {
		dbStatus = "error: " + err.Error()
	} else if err := sqlDB.PingContext(ctx); err != nil {
		dbStatus = "error: " + err.Error()
	}

	return connect.NewResponse(&healthifacev1.PingResponse{
		Status: "ok",
		Db:     dbStatus,
	}), nil
}
