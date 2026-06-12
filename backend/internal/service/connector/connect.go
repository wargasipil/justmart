package connector

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	connectorifacev1 "github.com/justmart/backend/gen/connector_iface/v1"
)

// Connect is the long-lived outbound stream from a connector. It is a `public`
// RPC (the unary auth interceptor doesn't cover streams), so it authenticates
// itself against the shared connector token, registers the device, acks, then
// blocks until the connector disconnects.
func (s *ConnectorService) Connect(
	ctx context.Context,
	req *connect.Request[connectorifacev1.ConnectRequest],
	stream *connect.ServerStream[connectorifacev1.ServerEvent],
) error {
	if err := s.authenticate(req.Msg.Token); err != nil {
		return err
	}
	id := req.Msg.DeviceId
	if id == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("device_id is required"))
	}

	c := &conn{
		deviceID:   id,
		deviceName: req.Msg.DeviceName,
		printers:   req.Msg.PrinterNames,
		send:       stream.Send,
	}
	s.register(c)
	defer s.unregister(c)

	c.sendMu.Lock()
	err := c.send(&connectorifacev1.ServerEvent{
		Event: &connectorifacev1.ServerEvent_Registered{
			Registered: &connectorifacev1.Registered{DeviceId: id},
		},
	})
	c.sendMu.Unlock()
	if err != nil {
		return err
	}

	// Hold the stream open until the connector disconnects (request ctx cancels).
	<-ctx.Done()
	return nil
}

// authenticate verifies the connector's shared token. An empty server token
// rejects everything (the connector feature is effectively off until a token is
// configured). Extracted for unit testing.
func (s *ConnectorService) authenticate(token string) error {
	if s.token == "" || token != s.token {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("invalid connector token"))
	}
	return nil
}

// register adds c to the registry, replacing any existing conn with the same id
// (a reconnect supersedes the stale one).
func (s *ConnectorService) register(c *conn) {
	s.mu.Lock()
	s.conns[c.deviceID] = c
	s.mu.Unlock()
}

// unregister removes c, but only if it's still the current conn for its id — a
// newer reconnect may have replaced it, and that one must survive.
func (s *ConnectorService) unregister(c *conn) {
	s.mu.Lock()
	if s.conns[c.deviceID] == c {
		delete(s.conns, c.deviceID)
	}
	s.mu.Unlock()
}
