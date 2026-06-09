package purchasing

import (
	"context"

	"connectrpc.com/connect"

	purchasingifacev1 "github.com/justmart/backend/gen/purchasing_iface/v1"
)

func (p *PurchasePayments) GetSupplierBalances(
	ctx context.Context,
	req *connect.Request[purchasingifacev1.GetSupplierBalancesRequest],
) (*connect.Response[purchasingifacev1.GetSupplierBalancesResponse], error) {
	type row struct {
		SupplierID   string `gorm:"column:supplier_id"`
		SupplierName string `gorm:"column:supplier_name"`
		Ordered      int64  `gorm:"column:ordered_total"`
		Paid         int64  `gorm:"column:paid_total"`
		Outstanding  int64
		OpenPOCount  int32 `gorm:"column:open_po_count"`
	}
	var rows []row
	q := p.db.WithContext(ctx).
		Table("suppliers s").
		Joins("LEFT JOIN purchase_orders po ON po.supplier_id = s.id AND po.status != ?", poStatusVoided).
		Select(`s.id AS supplier_id,
		        s.name AS supplier_name,
		        COALESCE(SUM(po.ordered_total), 0) AS ordered_total,
		        COALESCE(SUM(po.paid_amount), 0) AS paid_total,
		        COALESCE(SUM(po.ordered_total - po.paid_amount), 0) AS outstanding,
		        COUNT(po.id) FILTER (WHERE po.status NOT IN (?, ?)) AS open_po_count`,
			poStatusClosed, poStatusVoided).
		Group("s.id, s.name").
		Where("s.active = ?", true)
	if req.Msg.OnlyOutstanding {
		q = q.Having("COALESCE(SUM(po.ordered_total - po.paid_amount), 0) > 0")
	}
	q = q.Order("outstanding DESC, s.name ASC")

	if err := q.Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*purchasingifacev1.SupplierBalance, 0, len(rows))
	for _, r := range rows {
		out = append(out, &purchasingifacev1.SupplierBalance{
			SupplierId:   r.SupplierID,
			SupplierName: r.SupplierName,
			OrderedTotal: r.Ordered,
			PaidTotal:    r.Paid,
			Outstanding:  r.Outstanding,
			OpenPoCount:  r.OpenPOCount,
		})
	}
	return connect.NewResponse(&purchasingifacev1.GetSupplierBalancesResponse{Balances: out}), nil
}
