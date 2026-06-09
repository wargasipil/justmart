package product

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
)

// syncProductUnits upserts a product's units inside a tx: the base unit is
// derived from med.Unit/med.UnitPrice (factor 1); `inputs` are the larger
// (non-base) units. Non-base units absent from `inputs` are deactivated.
func syncProductUnits(tx *gorm.DB, med *model.Product, inputs []*inventoryifacev1.ProductUnitInput, changedBy string) error {
	// Upsert the base unit.
	var base model.ProductUnit
	err := tx.Where("product_id = ? AND is_base", med.ID).First(&base).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		base = model.ProductUnit{
			ProductID: med.ID, Name: med.Unit, Factor: 1, IsBase: true,
			SellPrice: med.UnitPrice, Sellable: true, Purchasable: true, Active: true,
		}
		if err := tx.Create(&base).Error; err != nil {
			return err
		}
		if err := recordUnitPrice(tx, base.ID, med.UnitPrice, changedBy); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if err := tx.Model(&model.ProductUnit{}).Where("id = ?", base.ID).
			Updates(map[string]any{"name": med.Unit, "sell_price": med.UnitPrice, "active": true}).Error; err != nil {
			return err
		}
		if base.SellPrice != med.UnitPrice {
			if err := recordUnitPrice(tx, base.ID, med.UnitPrice, changedBy); err != nil {
				return err
			}
		}
	}

	keptIDs := []string{base.ID}
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("unit name required"))
		}
		if in.Factor <= 1 {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unit %q factor must be > 1", name))
		}
		if in.SellPrice < 0 {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("sell_price must be >= 0"))
		}
		if in.Id != "" {
			var existing model.ProductUnit
			if err := tx.Where("id = ? AND product_id = ?", in.Id, med.ID).First(&existing).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ProductUnit{}).
				Where("id = ? AND product_id = ?", in.Id, med.ID).
				Updates(map[string]any{
					"name": name, "factor": in.Factor, "sell_price": in.SellPrice,
					"sellable": in.Sellable, "purchasable": in.Purchasable,
					"sort_order": int(in.SortOrder), "active": true,
				}).Error; err != nil {
				return err
			}
			if existing.SellPrice != in.SellPrice {
				if err := recordUnitPrice(tx, in.Id, in.SellPrice, changedBy); err != nil {
					return err
				}
			}
			keptIDs = append(keptIDs, in.Id)
		} else {
			row := model.ProductUnit{
				ProductID: med.ID, Name: name, Factor: in.Factor, IsBase: false,
				SellPrice: in.SellPrice, Sellable: in.Sellable, Purchasable: in.Purchasable,
				SortOrder: int(in.SortOrder), Active: true,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			if err := recordUnitPrice(tx, row.ID, in.SellPrice, changedBy); err != nil {
				return err
			}
			keptIDs = append(keptIDs, row.ID)
		}
	}
	// Deactivate non-base units that were removed from the set.
	return tx.Model(&model.ProductUnit{}).
		Where("product_id = ? AND is_base = false AND id NOT IN ?", med.ID, keptIDs).
		Update("active", false).Error
}

// recordUnitPrice closes a unit's open price row (if any) and inserts a new open
// row, mirroring the product_prices versioning for the base price.
func recordUnitPrice(tx *gorm.DB, unitID string, newPrice int64, changedBy string) error {
	now := time.Now()
	if err := tx.Model(&model.ProductUnitPrice{}).
		Where("product_unit_id = ? AND effective_to IS NULL", unitID).
		Update("effective_to", now).Error; err != nil {
		return err
	}
	row := model.ProductUnitPrice{
		ProductUnitID: unitID,
		UnitSellPrice: newPrice,
		EffectiveFrom: now,
	}
	if changedBy != "" {
		row.ChangedBy = &changedBy
	}
	return tx.Create(&row).Error
}
