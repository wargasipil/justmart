package product

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
)

// maxImportProducts bounds a single import request (one-shop migration fits well
// under this; the client can chunk if it ever needs more).
const maxImportProducts = 5000

// errDuplicateSKU marks a row whose SKU already exists — reported as SKIPPED, not
// an error (create-only import; re-running a fixed file is safe).
var errDuplicateSKU = errors.New("a product with this SKU already exists")

// ImportProducts bulk-creates products from a parsed CSV (first-time offline data
// migration). Best-effort: each row is processed in its own transaction so one
// bad row never blocks the rest. Existing SKUs are SKIPPED, invalid rows ERROR,
// valid rows CREATED — all reported per-row in input order.
func (s *ProductService) ImportProducts(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ImportProductsRequest],
) (*connect.Response[inventoryifacev1.ImportProductsResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	rows := req.Msg.Products
	if len(rows) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no products to import"))
	}
	if len(rows) > maxImportProducts {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("too many rows: %d (max %d)", len(rows), maxImportProducts))
	}

	results := make([]*inventoryifacev1.ImportProductResult, 0, len(rows))
	var created, skipped, errored int32

	for i, p := range rows {
		res := &inventoryifacev1.ImportProductResult{Row: int32(i), Sku: strings.TrimSpace(p.Sku)}

		if verr := validateCreate(p); verr != nil {
			res.Status = inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_ERROR
			res.Message = importErrMessage(verr)
			errored++
			results = append(results, res)
			continue
		}

		var productID string
		txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Skip rows whose SKU already exists (engine-agnostic pre-check; the
			// DB UNIQUE index still backstops a race).
			var existing model.Product
			lookup := tx.Where("sku = ?", res.Sku).First(&existing).Error
			if lookup == nil {
				return errDuplicateSKU
			}
			if !errors.Is(lookup, gorm.ErrRecordNotFound) {
				return lookup
			}
			m, cerr := createProductTx(tx, p, caller.UserID)
			if cerr != nil {
				return cerr
			}
			productID = m.ID
			return nil
		})

		switch {
		case txErr == nil:
			res.Status = inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_CREATED
			res.ProductId = productID
			created++
		case errors.Is(txErr, errDuplicateSKU):
			res.Status = inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_SKIPPED_DUPLICATE
			skipped++
		default:
			res.Status = inventoryifacev1.ImportProductStatus_IMPORT_PRODUCT_STATUS_ERROR
			res.Message = importErrMessage(txErr)
			errored++
		}
		results = append(results, res)
	}

	return connect.NewResponse(&inventoryifacev1.ImportProductsResponse{
		Results: results,
		Created: created,
		Skipped: skipped,
		Errored: errored,
	}), nil
}

// importErrMessage extracts a human-readable reason from a row error (the connect
// message when present, else the raw error).
func importErrMessage(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Message()
	}
	return err.Error()
}
