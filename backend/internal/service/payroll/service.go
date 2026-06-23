// Package payroll implements payroll_iface.v1.EmployeeService and
// payroll_iface.v1.PayrollService (employee roster + monthly payroll runs +
// payslips). One RPC per file; this file holds the structs + constructors.
// Shared helpers come from service/common; the statutory calculator lives in
// statutory.go.
package payroll

import (
	"gorm.io/gorm"

	"github.com/justmart/backend/internal/config"
	"github.com/justmart/backend/internal/service/common"
	"github.com/justmart/backend/internal/service/payment"
	"github.com/justmart/backend/internal/spooler"
)

const (
	runStatusDraft    = "DRAFT"
	runStatusApproved = "APPROVED"
	runStatusPaid     = "PAID"
	runStatusVoided   = "VOIDED"

	saleStatusCompleted = common.SaleStatusCompleted
)

// EmployeeService manages the staff roster (CRUD + search/resolve).
type EmployeeService struct {
	db *gorm.DB
}

func NewEmployeeService(db *gorm.DB) *EmployeeService { return &EmployeeService{db: db} }

// ConnectorPusher is the print-connector registry seam used by PrintPayslip when
// connector mode is on. *connector.ConnectorService satisfies it; tests pass a
// fake. Mirrors sale.ConnectorPusher (kept local so payroll doesn't import the
// connector package).
type ConnectorPusher interface {
	Push(deviceID, printerName string, payload []byte) (jobID string, err error)
}

// SpoolFunc prints rendered payslip bytes to a locally-installed printer (the
// usb-mode seam). Defaults to spooler.Print; tests inject a fake via SetSpooler.
type SpoolFunc func(printerName string, payload []byte, jobID string) error

// PayrollService runs the monthly payroll lifecycle (generate/list/get/edit/
// approve/pay/void) and prints payslips.
type PayrollService struct {
	db           *gorm.DB
	printer      config.Printer
	connectorCfg config.Connector
	connector    ConnectorPusher
	spool        SpoolFunc
	// disburser is the payment-integration abstraction. Payroll depends ONLY on
	// this interface (gateway-agnostic) — never on Xendit/Midtrans directly. nil
	// means no gateway is wired (manual payout only).
	disburser payment.Disburser
}

func NewPayrollService(db *gorm.DB, printerCfg config.Printer) *PayrollService {
	return &PayrollService{db: db, printer: printerCfg, spool: spooler.Print}
}

// SetDisburser wires the payment-integration layer used by MarkPayrollRunPaid in
// GATEWAY mode. Called once from serve.go (and from tests with a fake disburser).
func (s *PayrollService) SetDisburser(d payment.Disburser) { s.disburser = d }

// SetConnector wires the print-connector path (mirrors SaleService.SetConnector).
// Called once from serve.go after the registry is built (and from tests with a
// fake pusher).
func (s *PayrollService) SetConnector(connectorCfg config.Connector, pusher ConnectorPusher) {
	s.connectorCfg = connectorCfg
	s.connector = pusher
}

// SetSpooler overrides the usb-mode local print function (tests inject a fake).
func (s *PayrollService) SetSpooler(fn SpoolFunc) { s.spool = fn }
