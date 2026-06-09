package stock

import (
	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func movementToProto(m *model.StockMovement) *inventoryifacev1.StockMovement {
	return &inventoryifacev1.StockMovement{
		Id:        m.ID,
		BatchId:   m.BatchID,
		Qty:       m.Qty,
		Type:      movementTypeFromString(m.Type),
		Reason:    m.Reason,
		UserId:    m.UserID,
		CreatedAt: m.CreatedAt.Unix(),
	}
}

func movementTypeToString(t inventoryifacev1.MovementType) string {
	switch t {
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_PURCHASE:
		return "PURCHASE"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_SALE:
		return "SALE"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT:
		return "ADJUSTMENT"
	case inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF:
		return "WRITE_OFF"
	default:
		return ""
	}
}

func movementTypeFromString(s string) inventoryifacev1.MovementType {
	switch s {
	case "PURCHASE":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_PURCHASE
	case "SALE":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_SALE
	case "ADJUSTMENT":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_ADJUSTMENT
	case "WRITE_OFF":
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_WRITE_OFF
	default:
		return inventoryifacev1.MovementType_MOVEMENT_TYPE_UNSPECIFIED
	}
}
