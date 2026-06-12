package connector

import (
	"context"

	"connectrpc.com/connect"

	connectorifacev1 "github.com/justmart/backend/gen/connector_iface/v1"
)

// ListConnectors returns the connectors currently connected + their printers,
// for the Settings ▸ Printing picker.
func (s *ConnectorService) ListConnectors(
	_ context.Context,
	_ *connect.Request[connectorifacev1.ListConnectorsRequest],
) (*connect.Response[connectorifacev1.ListConnectorsResponse], error) {
	return connect.NewResponse(&connectorifacev1.ListConnectorsResponse{
		Connectors: s.List(),
	}), nil
}

// List returns a snapshot of the connected connectors (used by ListConnectors
// and by SaleService to resolve the printer list for a chosen device).
func (s *ConnectorService) List() []*connectorifacev1.Connector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*connectorifacev1.Connector, 0, len(s.conns))
	for _, c := range s.conns {
		out = append(out, &connectorifacev1.Connector{
			DeviceId:     c.deviceID,
			DeviceName:   c.deviceName,
			PrinterNames: append([]string(nil), c.printers...),
		})
	}
	return out
}
