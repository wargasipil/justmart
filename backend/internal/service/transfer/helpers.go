package transfer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	warehouseifacev1 "github.com/justmart/backend/gen/warehouse_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

func (s *TransferService) loadTransfer(ctx context.Context, id string) (*warehouseifacev1.StockTransfer, error) {
	var row model.StockTransfer
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("transfer not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return s.hydrateTransfer(ctx, &row, true)
}

func (s *TransferService) hydrateTransfer(
	ctx context.Context,
	row *model.StockTransfer,
	withLines bool,
) (*warehouseifacev1.StockTransfer, error) {
	out := &warehouseifacev1.StockTransfer{
		Id:              row.ID,
		FromWarehouseId: row.FromWarehouseID,
		ToWarehouseId:   row.ToWarehouseID,
		Note:            row.Note,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt.Unix(),
	}
	if row.TransferNo != nil {
		out.TransferNo = *row.TransferNo
	}
	// Warehouse names.
	var whs []model.Warehouse
	if err := s.db.WithContext(ctx).Where("id IN ?", []string{row.FromWarehouseID, row.ToWarehouseID}).
		Find(&whs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, w := range whs {
		if w.ID == row.FromWarehouseID {
			out.FromWarehouseName = w.Name
		}
		if w.ID == row.ToWarehouseID {
			out.ToWarehouseName = w.Name
		}
	}
	if !withLines {
		return out, nil
	}
	// Reconstruct lines from the TRANSFER_IN movements (one per batch moved).
	type lineRow struct {
		BatchID     string `gorm:"column:batch_id"`
		Qty         int32  `gorm:"column:qty"`
		ProductID   string `gorm:"column:product_id"`
		ProductName string `gorm:"column:product_name"`
		BatchNumber string `gorm:"column:batch_number"`
		ExpiryDate  string `gorm:"column:expiry_date"`
	}
	var rows []lineRow
	err := s.db.WithContext(ctx).
		Table("stock_movements AS m").
		Select(`m.batch_id, m.qty,
		        b.product_id AS product_id,
		        med.name AS product_name,
		        b.batch_number AS batch_number,
		        `+common.DayKeyExpr(s.db, "b.expiry_date")+` AS expiry_date`).
		Joins("JOIN batches AS b ON b.id = m.batch_id").
		Joins("JOIN products AS med ON med.id = b.product_id").
		Where("m.transfer_id = ? AND m.type = ?", row.ID, movementTransferIn).
		Order("med.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range rows {
		out.Lines = append(out.Lines, &warehouseifacev1.StockTransferLine{
			BatchId:     r.BatchID,
			ProductId:   r.ProductID,
			ProductName: r.ProductName,
			BatchNumber: r.BatchNumber,
			ExpiryDate:  r.ExpiryDate,
			Qty:         r.Qty,
		})
	}
	return out, nil
}

func assignTransferNo(tx *gorm.DB, now time.Time) (string, error) {
	year := now.Year()
	var counter model.TransferCounter
	err := tx.Where("year = ?", year).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = model.TransferCounter{Year: year, LastSeq: 0}
		if err := tx.Create(&counter).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := tx.Model(&model.TransferCounter{}).
		Where("year = ?", year).
		Update("last_seq", gorm.Expr("last_seq + 1")).Error; err != nil {
		return "", err
	}
	if err := tx.Where("year = ?", year).First(&counter).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("TRF-%d-%04d", year, counter.LastSeq), nil
}
