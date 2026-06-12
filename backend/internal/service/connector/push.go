package connector

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	connectorifacev1 "github.com/justmart/backend/gen/connector_iface/v1"
)

// Push sends a render-ready ESC/POS payload to a connected connector and returns
// the generated job id. deviceID "" selects the sole connected connector (the
// common single-connector case). It is NOT an RPC — SaleService.PrintReceipt
// calls it. The send is serialized per-conn (connect-go streams aren't
// concurrency-safe) and runs on a different goroutine than the one blocked in
// Connect, hence the per-conn sendMu.
func (s *ConnectorService) Push(deviceID, printerName string, payload []byte) (string, error) {
	s.mu.RLock()
	c, ok := s.resolve(deviceID)
	s.mu.RUnlock()
	if !ok {
		if deviceID == "" {
			return "", connect.NewError(connect.CodeUnavailable, errors.New("no print connector is connected"))
		}
		return "", connect.NewError(connect.CodeUnavailable, fmt.Errorf("print connector %q is not connected", deviceID))
	}

	jobID := uuid.NewString()
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if err := c.send(&connectorifacev1.ServerEvent{
		Event: &connectorifacev1.ServerEvent_PrintJob{
			PrintJob: &connectorifacev1.PrintJob{JobId: jobID, PrinterName: printerName, Payload: payload},
		},
	}); err != nil {
		return "", connect.NewError(connect.CodeUnavailable, err)
	}
	return jobID, nil
}

// resolve returns the conn for deviceID, or — when deviceID is "" and exactly
// one connector is connected — that sole connector. Caller holds s.mu (read).
func (s *ConnectorService) resolve(deviceID string) (*conn, bool) {
	if deviceID != "" {
		c, ok := s.conns[deviceID]
		return c, ok
	}
	if len(s.conns) == 1 {
		for _, c := range s.conns {
			return c, true
		}
	}
	return nil, false
}
