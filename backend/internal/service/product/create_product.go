package product

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

func (s *ProductService) CreateProduct(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.CreateProductRequest],
) (*connect.Response[inventoryifacev1.CreateProductResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateCreate(req.Msg); err != nil {
		return nil, err
	}

	var med *model.Product
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		m, err := createProductTx(tx, req.Msg, caller.UserID)
		if err != nil {
			return err
		}
		med = m
		return nil
	})
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			return nil, err // unit validation error — keep its code
		}
		return nil, connect.NewError(connect.CodeAlreadyExists, err) // likely dup SKU
	}
	out := productToProto(med)
	if err := s.attachUnits(ctx, []*inventoryifacev1.Product{out}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&inventoryifacev1.CreateProductResponse{Product: out}), nil
}

// validateCreate checks the shared CreateProduct field rules (sku/name/unit
// required, unit_price >= 0). Used by both CreateProduct and ImportProducts so
// validation is identical. Returns InvalidArgument with a clear message.
func validateCreate(msg *inventoryifacev1.CreateProductRequest) error {
	if strings.TrimSpace(msg.Sku) == "" || strings.TrimSpace(msg.Name) == "" || strings.TrimSpace(msg.Unit) == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("sku, name, unit required"))
	}
	if msg.UnitPrice < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("unit_price must be >= 0"))
	}
	return nil
}

// createProductTx creates the product row + initial price-version + units inside
// tx. Assumes validateCreate already passed. Returns the created product. Shared
// by CreateProduct (one tx) and ImportProducts (one tx per row).
func createProductTx(tx *gorm.DB, msg *inventoryifacev1.CreateProductRequest, userID string) (*model.Product, error) {
	med := &model.Product{
		SKU:                  strings.TrimSpace(msg.Sku),
		Name:                 strings.TrimSpace(msg.Name),
		Unit:                 strings.TrimSpace(msg.Unit),
		UnitPrice:            msg.UnitPrice,
		PrescriptionRequired: msg.PrescriptionRequired,
		Active:               true,
	}
	if err := tx.Create(med).Error; err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	price := model.ProductPrice{
		ProductID:     med.ID,
		UnitPrice:     med.UnitPrice,
		EffectiveFrom: time.Now(),
		ChangedBy:     userID,
	}
	if err := tx.Create(&price).Error; err != nil {
		return nil, fmt.Errorf("create initial price: %w", err)
	}
	// Base unit (factor 1) + any additional units supplied.
	if err := syncProductUnits(tx, med, msg.Units, userID); err != nil {
		return nil, err
	}
	return med, nil
}
