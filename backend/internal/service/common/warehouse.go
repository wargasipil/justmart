package common

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

// ResolveWarehouse returns the active warehouse id for the caller: the
// X-Warehouse-Id header if present, else the user's default membership, else
// the global default warehouse. It never returns empty on a healthy DB, so
// every stock movement can be stamped with a concrete warehouse.
func ResolveWarehouse(ctx context.Context, db *gorm.DB, caller auth.Principal) (string, error) {
	if caller.WarehouseID != "" {
		return caller.WarehouseID, nil
	}
	var mem model.UserWarehouse
	err := db.WithContext(ctx).Where("user_id = ? AND is_default", caller.UserID).First(&mem).Error
	if err == nil {
		return mem.WarehouseID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	var def model.Warehouse
	if err := db.WithContext(ctx).Where("is_default").First(&def).Error; err != nil {
		return "", connect.NewError(connect.CodeFailedPrecondition,
			errors.New("no warehouse configured"))
	}
	return def.ID, nil
}
