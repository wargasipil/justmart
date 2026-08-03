package supplier

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gorm.io/gorm"

	inventoryifacev1 "github.com/justmart/backend/gen/inventory_iface/v1"
	"github.com/justmart/backend/internal/model"
	"github.com/justmart/backend/internal/service/common"
)

// supplierFilters is the one predicate behind both ListSuppliers and
// GetSuppliersSummary. Shared rather than copy-pasted on purpose: the stat row
// above the table must describe exactly the set the table pages through, and
// two hand-kept-in-sync WHERE clauses is how that silently stops being true.
func supplierFilters(includeInactive bool, query string) func(*gorm.DB) *gorm.DB {
	query = strings.TrimSpace(query)
	return func(q *gorm.DB) *gorm.DB {
		if !includeInactive {
			q = q.Where("active = ?", true)
		}
		if query != "" {
			pattern := "%" + query + "%"
			q = q.Where("name "+common.LikeOp(q)+" ? OR code "+common.LikeOp(q)+" ?", pattern, pattern)
		}
		return q
	}
}

func (s *SupplierService) load(ctx context.Context, id string) (*model.Supplier, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var sup model.Supplier
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&sup).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("supplier %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &sup, nil
}

func supplierToProto(s *model.Supplier) *inventoryifacev1.Supplier {
	return &inventoryifacev1.Supplier{
		Id:                s.ID,
		Code:              s.Code,
		Name:              s.Name,
		ContactEmail:      s.ContactEmail,
		Phone:             s.Phone,
		Address:           s.Address,
		BankName:          s.BankName,
		BankAccountNumber: s.BankAccountNumber,
		BankAccountHolder: s.BankAccountHolder,
		Active:            s.Active,
		CreatedAt:         s.CreatedAt.Unix(),
	}
}
