package batch

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/auth"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// maxImportStockRows bounds a single import request (one-shop migration fits well
// under this; the client can chunk if it ever needs more).
const maxImportStockRows = 5000

// farFutureExpiry is the sentinel expiry for non-expiring goods (blank CSV cell).
var farFutureExpiry = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

// errBatchExists marks a row whose (product, batch_number) already exists —
// reported as SKIPPED (so re-importing a file with batch numbers is safe).
var errBatchExists = errors.New("a batch with this number already exists for the product")

// ImportStock bulk-creates opening-stock batches from a parsed CSV (first-time
// offline inventory migration). SKU-keyed, best-effort: each row is its own
// transaction so one bad row never blocks the rest. Each created row makes one
// batch + a PURCHASE movement of the (base-unit) quantity into the active
// warehouse. Per-row outcomes: CREATED / SKIPPED_EXISTS / ERROR.
func (s *BatchService) ImportStock(
	ctx context.Context,
	req *connect.Request[inventoryifacev1.ImportStockRequest],
) (*connect.Response[inventoryifacev1.ImportStockResponse], error) {
	caller, err := auth.MustPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	rows := req.Msg.Rows
	if len(rows) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no rows to import"))
	}
	if len(rows) > maxImportStockRows {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("too many rows: %d (max %d)", len(rows), maxImportStockRows))
	}
	warehouseID, err := common.ResolveWarehouse(ctx, s.db, caller)
	if err != nil {
		return nil, err
	}

	results := make([]*inventoryifacev1.ImportStockResult, 0, len(rows))
	var created, skipped, errored int32

	for i, row := range rows {
		res := &inventoryifacev1.ImportStockResult{Row: int32(i), Sku: strings.TrimSpace(row.Sku)}

		if vErr := validateStockRow(row); vErr != nil {
			res.Status = inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR
			res.Message = stockErrMessage(vErr)
			errored++
			results = append(results, res)
			continue
		}

		// Expiry: blank => non-expiring sentinel.
		expiry := farFutureExpiry
		if ed := strings.TrimSpace(row.ExpiryDate); ed != "" {
			expiry, err = time.Parse(common.DateLayout, ed)
			if err != nil {
				res.Status = inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR
				res.Message = "expiry_date must be YYYY-MM-DD"
				errored++
				results = append(results, res)
				continue
			}
		}

		var batchID string
		txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var product model.Product
			pErr := tx.Where("sku = ?", res.Sku).First(&product).Error
			if errors.Is(pErr, gorm.ErrRecordNotFound) {
				return errProductNotFound
			}
			if pErr != nil {
				return pErr
			}

			// Resolve the entry unit → base-unit quantity.
			baseQty := row.Quantity
			if u := strings.TrimSpace(row.Unit); u != "" {
				var pu model.ProductUnit
				uErr := tx.Where("product_id = ? AND name = ? AND active", product.ID, u).First(&pu).Error
				if errors.Is(uErr, gorm.ErrRecordNotFound) {
					return fmt.Errorf("unit %q not found for this product", u)
				}
				if uErr != nil {
					return uErr
				}
				factor := pu.Factor
				if factor < 1 {
					factor = 1
				}
				baseQty = row.Quantity * factor
			}
			if baseQty <= 0 || baseQty > math.MaxInt32 {
				return fmt.Errorf("resulting quantity %d out of range", baseQty)
			}

			batchNo := strings.TrimSpace(row.BatchNumber)
			if batchNo != "" {
				var existing model.Batch
				bErr := tx.Where("product_id = ? AND batch_number = ?", product.ID, batchNo).First(&existing).Error
				if bErr == nil {
					return errBatchExists
				}
				if !errors.Is(bErr, gorm.ErrRecordNotFound) {
					return bErr
				}
			}

			b := model.Batch{
				ProductID:   product.ID,
				BatchNumber: batchNo,
				ExpiryDate:  expiry,
				CostPrice:   row.CostPrice,
				ReceivedAt:  time.Now(),
			}
			if err := tx.Create(&b).Error; err != nil {
				return fmt.Errorf("create batch: %w", err)
			}
			mv := model.StockMovement{
				BatchID:     b.ID,
				Qty:         int32(baseQty),
				Type:        "PURCHASE",
				Reason:      "Opening stock (import)",
				UserID:      caller.UserID,
				WarehouseID: warehouseID,
			}
			if err := tx.Create(&mv).Error; err != nil {
				return fmt.Errorf("create movement: %w", err)
			}
			batchID = b.ID
			return nil
		})

		switch {
		case txErr == nil:
			res.Status = inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_CREATED
			res.BatchId = batchID
			created++
		case errors.Is(txErr, errBatchExists):
			res.Status = inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_SKIPPED_EXISTS
			skipped++
		default:
			res.Status = inventoryifacev1.ImportStockStatus_IMPORT_STOCK_STATUS_ERROR
			res.Message = stockErrMessage(txErr)
			errored++
		}
		results = append(results, res)
	}

	return connect.NewResponse(&inventoryifacev1.ImportStockResponse{
		Results: results,
		Created: created,
		Skipped: skipped,
		Errored: errored,
	}), nil
}

var errProductNotFound = errors.New("product not found for this SKU")

func validateStockRow(row *inventoryifacev1.ImportStockRow) error {
	if strings.TrimSpace(row.Sku) == "" {
		return errors.New("sku required")
	}
	if row.Quantity <= 0 {
		return errors.New("quantity must be > 0")
	}
	if row.CostPrice < 0 {
		return errors.New("cost must be >= 0")
	}
	return nil
}

func stockErrMessage(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Message()
	}
	return err.Error()
}
