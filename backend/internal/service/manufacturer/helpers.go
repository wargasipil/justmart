package manufacturer

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

// manufacturerFilters is the one predicate behind BOTH ListManufacturers and
// GetManufacturersSummary. Shared rather than copy-pasted: the stat tile above
// the table must describe exactly the set the table pages through, and two
// hand-kept-in-sync WHERE clauses is how that silently stops being true.
// TestGetManufacturersSummary_HonorsListFilters pins it.
func manufacturerFilters(includeInactive bool, query string) func(*gorm.DB) *gorm.DB {
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

func (s *ManufacturerService) load(ctx context.Context, id string) (*model.Manufacturer, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id required"))
	}
	var m model.Manufacturer
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("manufacturer %s not found", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &m, nil
}

func manufacturerToProto(m *model.Manufacturer) *inventoryifacev1.Manufacturer {
	return &inventoryifacev1.Manufacturer{
		Id:           m.ID,
		Code:         m.Code,
		Name:         m.Name,
		Address:      m.Address,
		Phone:        m.Phone,
		ContactEmail: m.ContactEmail,
		Note:         m.Note,
		Active:       m.Active,
		CreatedAt:    m.CreatedAt.Unix(),
	}
}

// assertUnique returns the specific field token for a colliding code or active
// name. `excludeID` skips the row being updated. Engine-agnostic pre-check
// (common.ExistsBy) so the client gets a field error rather than raw DB text;
// the unique indexes backstop the race with the same tokens.
func assertUnique(db *gorm.DB, code, name, excludeID string) error {
	codeWhere, codeArgs := "code = ?", []any{code}
	nameWhere, nameArgs := "name = ? AND active = ?", []any{name, true}
	if excludeID != "" {
		codeWhere += " AND id <> ?"
		codeArgs = append(codeArgs, excludeID)
		nameWhere += " AND id <> ?"
		nameArgs = append(nameArgs, excludeID)
	}
	if taken, err := common.ExistsBy(db, &model.Manufacturer{}, codeWhere, codeArgs...); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	} else if taken {
		return common.TokenError(connect.CodeAlreadyExists, "manufacturer.code_taken")
	}
	if taken, err := common.ExistsBy(db, &model.Manufacturer{}, nameWhere, nameArgs...); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	} else if taken {
		return common.TokenError(connect.CodeAlreadyExists, "manufacturer.name_taken")
	}
	return nil
}
