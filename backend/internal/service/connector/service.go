// Package connector implements connector_iface.v1.ConnectorService — the server
// side of the print-connector link. A connector is a separate (Windows) program
// running next to the physical printer; it dials Connect (outbound), registers
// its installed printers, and receives PrintJob pushes for the life of the
// connection. The registry is in-memory + single-node (same posture as the
// login rate limiter and the draft sweeper).
package connector

import (
	"sync"

	connectorifacev1 "github.com/justmart/backend/gen/connector_iface/v1"
	"github.com/justmart/backend/gen/connector_iface/v1/connectorifacev1connect"
)

// conn is one connected connector holding its open server stream's send func.
// `send` is abstracted (not the raw *connect.ServerStream) so the registry is
// unit-testable with a fake send. connect-go streams are NOT safe for
// concurrent Send, so every send goes through sendMu.
type conn struct {
	deviceID   string
	deviceName string
	printers   []string
	sendMu     sync.Mutex
	send       func(*connectorifacev1.ServerEvent) error
}

// ConnectorService holds the live device registry, guarded by mu. Connectors
// connect freely (no auth) — see the Connect handler.
type ConnectorService struct {
	mu    sync.RWMutex
	conns map[string]*conn
}

func NewConnectorService() *ConnectorService {
	return &ConnectorService{conns: map[string]*conn{}}
}

var _ connectorifacev1connect.ConnectorServiceHandler = (*ConnectorService)(nil)
