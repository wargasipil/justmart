// White-box (package connector) so the tests can construct a fake conn with a
// channel-backed send — a real *connect.ServerStream isn't unit-constructable.
// No DB → engine-agnostic; identical on sqlite + postgres.
package connector

import (
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	connectorifacev1 "github.com/justmart/backend/gen/connector_iface/v1"
)

func fakeConn(id string, ch chan *connectorifacev1.ServerEvent) *conn {
	return &conn{
		deviceID:   id,
		deviceName: id + "-pc",
		printers:   []string{"POS-58"},
		send:       func(ev *connectorifacev1.ServerEvent) error { ch <- ev; return nil },
	}
}

func TestPush_DeliversToRegisteredConn(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("tok")
	ch := make(chan *connectorifacev1.ServerEvent, 4)
	c := fakeConn("dev1", ch)
	s.register(c)

	jobID, err := s.Push("dev1", "POS-58", []byte("hello"))
	require.NoError(t, err)
	require.NotEmpty(t, jobID)

	pj := (<-ch).GetPrintJob()
	require.NotNil(t, pj)
	require.Equal(t, jobID, pj.JobId)
	require.Equal(t, "POS-58", pj.PrinterName)
	require.Equal(t, []byte("hello"), pj.Payload)

	list := s.List()
	require.Len(t, list, 1)
	require.Equal(t, "dev1", list[0].DeviceId)
	require.Equal(t, []string{"POS-58"}, list[0].PrinterNames)

	// After unregister, the same push is Unavailable.
	s.unregister(c)
	_, err = s.Push("dev1", "POS-58", []byte("x"))
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestPush_EmptyDeviceUsesSoleConnector(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("tok")
	ch := make(chan *connectorifacev1.ServerEvent, 1)
	s.register(fakeConn("only", ch))

	_, err := s.Push("", "", []byte("z")) // empty deviceID → the one connector
	require.NoError(t, err)
	require.NotNil(t, (<-ch).GetPrintJob())
}

func TestPush_EmptyDeviceAmbiguous(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("tok")
	s.register(fakeConn("a", make(chan *connectorifacev1.ServerEvent, 1)))
	s.register(fakeConn("b", make(chan *connectorifacev1.ServerEvent, 1)))

	_, err := s.Push("", "", []byte("z")) // 2 connectors, no id → Unavailable
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestPush_NotConnected(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("tok")
	_, err := s.Push("ghost", "", nil)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestReconnect_SameIDReplacesAndSurvivesStaleUnregister(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("tok")
	ch1 := make(chan *connectorifacev1.ServerEvent, 1)
	ch2 := make(chan *connectorifacev1.ServerEvent, 1)
	old := fakeConn("dev", ch1)
	fresh := fakeConn("dev", ch2)

	s.register(old)
	s.register(fresh)  // reconnect supersedes old
	s.unregister(old)  // stale unregister must NOT evict fresh

	_, err := s.Push("dev", "", []byte("y"))
	require.NoError(t, err)
	require.NotNil(t, (<-ch2).GetPrintJob()) // delivered to fresh, not old
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("secret")
	require.NoError(t, s.authenticate("secret"))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(s.authenticate("wrong")))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(s.authenticate("")))
	// Empty server token rejects everything (feature off until configured).
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(NewConnectorService("").authenticate("anything")))
}

// Run with -race: pushing while unregistering must not panic or send-after-evict.
func TestPush_ConcurrentWithUnregister(t *testing.T) {
	t.Parallel()
	s := NewConnectorService("tok")
	ch := make(chan *connectorifacev1.ServerEvent, 100)
	c := fakeConn("dev", ch)
	s.register(c)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = s.Push("dev", "", []byte("x")) }()
	}
	s.unregister(c)
	wg.Wait()
}
