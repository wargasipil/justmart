package product

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// A product's approved-source list: who MAY make this item.
//
// Distinct from the fact on a lot (Batch.ManufacturerID, who DID make it).
// Both writers here go through the same two helpers so they cannot disagree
// about what an assignable pabrik is, or about the invariant that the primary
// is always a member of its own set.

// assertAssignable rejects a pabrik that must not be ADDED to a list.
//
// An archived one is refused for the same reason SearchManufacturers is
// active-only: a shop cannot start sourcing from a factory it has retired. A
// product already pointing at an archived maker KEEPS it (ResolveManufacturers
// includes archived rows, so the name still renders) -- archiving must not
// silently rewrite what was true when it was recorded.
func assertAssignable(tx *gorm.DB, id string) error {
	var maker model.Manufacturer
	err := tx.Where("id = ?", id).First(&maker).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("manufacturer %s not found", id))
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !maker.Active {
		return common.TokenError(connect.CodeFailedPrecondition, "manufacturer.archived")
	}
	return nil
}

// setPrimary points products.manufacturer_id at one of the product's approved
// makers, or clears it when the list is empty.
//
// ALWAYS writes the column, so an empty id CLEARS the link rather than being
// indistinguishable from "field omitted" -- the rule UpdateProduct follows.
func setPrimary(tx *gorm.DB, productID, manufacturerID string) error {
	return tx.Model(&model.Product{}).Where("id = ?", productID).
		Update("manufacturer_id", manufacturerRef(manufacturerID)).Error
}

// listPrimary records a product's primary maker as an approved source, so the
// invariant "the primary is a member of its own set" survives a write that only
// knows about the ONE manufacturer a product form carries.
//
// CreateProduct and UpdateProduct are the callers. Without this they set
// products.manufacturer_id while product_manufacturers stays empty, and the two
// readers that trust the invariant both go wrong: the restock line's picker
// offers an approved list missing the product's own usual maker, and the pabrik
// detail page (which lists by the M2M) cannot see the product at all.
//
// ADDITIVE ON PURPOSE -- it never removes. A product form holds one maker, so a
// replace there would silently drop every other approved source whenever
// somebody edited the usual one, which is precisely the overwrite this model
// was split to stop. Removal stays with SetProductManufacturers, where absence
// from a full set is what expresses it.
//
// An EMPTY id is a no-op rather than a removal, for the same reason: clearing
// "who usually makes this" does not retract permission to source from them.
func listPrimary(tx *gorm.DB, productID, manufacturerID string) error {
	if manufacturerID == "" {
		return nil
	}
	row := model.ProductManufacturer{ProductID: productID, ManufacturerID: manufacturerID}
	return tx.Where("product_id = ? AND manufacturer_id = ?", productID, manufacturerID).
		FirstOrCreate(&row).Error
}

// AddProductManufacturer approves one more pabrik for a product, idempotently.
//
// This is the RESTOCK FORM's writer: a buyer holding the distributor's invoice
// has just learned of a source the catalog does not list, and refusing them
// here -- behind an admin screen they cannot reach from a half-typed order --
// would mean the order records no maker at all, which is the outcome this whole
// feature exists to prevent.
func (s *ProductService) AddProductManufacturer(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.AddProductManufacturerRequest],
) (*connect.Response[inventoryifacev1.AddProductManufacturerResponse], error) {
	med, err := s.load(ctx, req.Msg.ProductId)
	if err != nil {
		return nil, err
	}
	if req.Msg.ManufacturerId == "" {
		return nil, common.TokenError(connect.CodeInvalidArgument, "manufacturer.required")
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := assertAssignable(tx, req.Msg.ManufacturerId); err != nil {
			return err
		}
		// Idempotent: re-adding an approved maker is a no-op, not an error. The
		// restock form cannot know what the list already holds without a round
		// trip, and a buyer picking the pabrik that is already recorded has done
		// nothing wrong.
		row := model.ProductManufacturer{ProductID: med.ID, ManufacturerID: req.Msg.ManufacturerId}
		if err := tx.Where("product_id = ? AND manufacturer_id = ?", med.ID, req.Msg.ManufacturerId).
			FirstOrCreate(&row).Error; err != nil {
			return err
		}
		// A first source is the usual one until someone says otherwise.
		if med.ManufacturerID == nil || *med.ManufacturerID == "" {
			return setPrimary(tx, med.ID, req.Msg.ManufacturerId)
		}
		return nil
	})
	if err != nil {
		return nil, wrapTxError(err)
	}

	out, err := s.reloadProto(ctx, med.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.AddProductManufacturerResponse{Product: out}), nil
}

// SetProductManufacturers replaces the whole approved-source list and names the
// primary. The product detail page's admin card; the full-set shape mirrors how
// units are edited, so a removal is expressed by absence rather than by a
// second RPC.
func (s *ProductService) SetProductManufacturers(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.SetProductManufacturersRequest],
) (*connect.Response[inventoryifacev1.SetProductManufacturersResponse], error) {
	med, err := s.load(ctx, req.Msg.ProductId)
	if err != nil {
		return nil, err
	}

	// Dedupe while preserving the caller's order: the first id is the fallback
	// primary, so the order it arrived in carries meaning.
	wanted := make([]string, 0, len(req.Msg.ManufacturerIds))
	seen := make(map[string]bool, len(req.Msg.ManufacturerIds))
	for _, id := range req.Msg.ManufacturerIds {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		wanted = append(wanted, id)
	}

	primary := req.Msg.PrimaryManufacturerId
	if primary != "" && !seen[primary] {
		// A primary outside its own list is the one inconsistency this RPC could
		// be asked to create, so it is the one it refuses.
		return nil, common.TokenError(connect.CodeInvalidArgument, "manufacturer.primary_not_listed")
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []model.ProductManufacturer
		if err := tx.Where("product_id = ?", med.ID).Find(&existing).Error; err != nil {
			return err
		}
		had := make(map[string]bool, len(existing))
		for _, e := range existing {
			had[e.ManufacturerID] = true
		}
		// Only a NEW id has to be assignable. An archived maker already on the
		// list survives a re-save of that list, so editing a product does not
		// force the operator to drop history they did not come here to touch.
		for _, id := range wanted {
			if had[id] {
				continue
			}
			if err := assertAssignable(tx, id); err != nil {
				return err
			}
			if err := tx.Create(&model.ProductManufacturer{
				ProductID: med.ID, ManufacturerID: id,
			}).Error; err != nil {
				return err
			}
		}
		if len(wanted) == 0 {
			if err := tx.Where("product_id = ?", med.ID).
				Delete(&model.ProductManufacturer{}).Error; err != nil {
				return err
			}
		} else if err := tx.Where("product_id = ? AND manufacturer_id NOT IN ?", med.ID, wanted).
			Delete(&model.ProductManufacturer{}).Error; err != nil {
			return err
		}

		// Resolve the primary LAST, against the list that now exists: an empty
		// request keeps the current primary when it survived the edit and falls
		// to the first remaining source when it did not. Dropping a product's
		// only maker therefore clears the pointer instead of dangling it.
		next := primary
		if next == "" {
			if med.ManufacturerID != nil && seen[*med.ManufacturerID] {
				next = *med.ManufacturerID
			} else if len(wanted) > 0 {
				next = wanted[0]
			}
		}
		return setPrimary(tx, med.ID, next)
	})
	if err != nil {
		return nil, wrapTxError(err)
	}

	out, err := s.reloadProto(ctx, med.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.SetProductManufacturersResponse{Product: out}), nil
}

// wrapTxError keeps a token/code error intact and wraps anything else, so a
// refusal reaches the client as itself rather than as a bare Aborted.
func wrapTxError(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeAborted, err)
}

// reloadProto re-reads a product and hydrates it the way every other write path
// answers: fresh from the DB, units and sources attached.
func (s *ProductService) reloadProto(ctx context.Context, id string) (*inventoryifacev1.Product, error) {
	med, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	out := productToProto(med)
	if err := s.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return out, nil
}
