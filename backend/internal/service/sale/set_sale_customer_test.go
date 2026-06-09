package sale_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	posifacev1 "github.com/justmart/backend/gen/pos_iface/v1"
	"github.com/justmart/backend/internal/model"
)

func TestSetSaleCustomer_HappyPath(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	cust := model.Customer{Name: "Siti", Active: true}
	require.NoError(t, db.Create(&cust).Error)
	saleID := startDraft(t, svc, ctx)

	resp, err := svc.SetSaleCustomer(ctx, connect.NewRequest(&posifacev1.SetSaleCustomerRequest{
		SaleId:     saleID,
		CustomerId: cust.ID,
	}))
	require.NoError(t, err)
	require.Equal(t, cust.ID, resp.Msg.Sale.CustomerId)
}

func TestSetSaleCustomer_ClearCustomer(t *testing.T) {
	t.Parallel()
	svc, ctx, db, _ := newSaleSvc(t)
	cust := model.Customer{Name: "Budi", Active: true}
	require.NoError(t, db.Create(&cust).Error)
	saleID := startDraft(t, svc, ctx)

	_, err := svc.SetSaleCustomer(ctx, connect.NewRequest(&posifacev1.SetSaleCustomerRequest{
		SaleId: saleID, CustomerId: cust.ID,
	}))
	require.NoError(t, err)

	// Empty customer id clears the attachment.
	resp, err := svc.SetSaleCustomer(ctx, connect.NewRequest(&posifacev1.SetSaleCustomerRequest{
		SaleId: saleID, CustomerId: "",
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Sale.CustomerId)
}

func TestSetSaleCustomer_SaleNotFound(t *testing.T) {
	t.Parallel()
	svc, ctx, _, _ := newSaleSvc(t)

	_, err := svc.SetSaleCustomer(ctx, connect.NewRequest(&posifacev1.SetSaleCustomerRequest{
		SaleId:     "00000000-0000-0000-0000-0000000000cc",
		CustomerId: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
