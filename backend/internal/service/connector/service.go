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

// ConnectorService holds the live device registry, guarded by mu.
//
// enabled gates the Connect STREAM. Connect is a `public`, UNAUTHENTICATED RPC
// and the auth/audit interceptors are UnaryInterceptorFunc — they cannot guard
// a stream. On a LAN that's fine, but on an internet-facing deploy (e.g. Fly)
// an open Connect would let anyone register as the shop's connector and receive
// every rendered receipt, evict the real connector, or hold streams to exhaust
// memory. So the stream is refused unless the server is actually running in
// connector print mode (connector.mode == "connector"). Fail-closed: the zero
// value is disabled — serve.go opts in explicitly.
type ConnectorService struct {
	mu      sync.RWMutex
	conns   map[string]*conn
	enabled bool
}

// NewConnectorService builds the registry. enabled must be true only when
// connector.mode == "connector"; otherwise the Connect stream is refused.
func NewConnectorService(enabled bool) *ConnectorService {
	return &ConnectorService{conns: map[string]*conn{}, enabled: enabled}
}

var _ connectorifacev1connect.ConnectorServiceHandler = (*ConnectorService)(nil)
