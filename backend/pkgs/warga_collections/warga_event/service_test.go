package warga_event

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	eventv1 "github.com/wargasipil/warga_collections/definitions/event_iface/v1"
	"github.com/wargasipil/warga_collections/definitions/event_iface/v1/event_ifacev1connect"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := Open(":memory:")
	require.NoError(t, err)
	return db
}

// newTestServer mounts the handler (with the protovalidate interceptor, as a
// real deployment would) behind an httptest server and returns a client. The
// underlying *gorm.DB is returned so tests can seed/inspect rows directly.
func newTestServer(t *testing.T, opts ...Option) (event_ifacev1connect.EventServiceClient, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	svc := NewEventService(db, opts...)

	interceptor := validate.NewInterceptor()

	mux := http.NewServeMux()
	path, h := event_ifacev1connect.NewEventServiceHandler(svc, connect.WithInterceptors(interceptor))
	mux.Handle(path, h)

	// Server-streaming RPCs (Pull/Maintain) are long-poll: the server flushes
	// response headers only on the first message, so the client's Pull call
	// would block until the first Send over HTTP/1.1. HTTP/2 decouples the
	// request and response streams, which is what a real deployment uses (h2c
	// or TLS), so the test server speaks HTTP/2.
	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	client := event_ifacev1connect.NewEventServiceClient(srv.Client(), srv.URL)
	return client, db
}

func TestSend_NoSubscription_RecordsTopicButDropsMessage(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	svc := NewEventService(db)

	resp, err := svc.Send(context.Background(), connect.NewRequest(&eventv1.SendRequest{
		Topic:   "orders",
		Message: &eventv1.Message{Data: []byte("hi")},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetMessageId())

	var topicCount, deliveryCount int64
	require.NoError(t, db.Model(&Topic{}).Where("name = ?", "orders").Count(&topicCount).Error)
	require.NoError(t, db.Model(&Delivery{}).Count(&deliveryCount).Error)
	require.Equal(t, int64(1), topicCount, "topic should be recorded")
	require.Equal(t, int64(0), deliveryCount, "no subscriptions -> no deliveries")
}

func TestSend_FanOutToAllSubscriptions(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	svc := NewEventService(db)
	ctx := context.Background()

	require.NoError(t, svc.ensureSubscription(ctx, "sub-a", "orders"))
	require.NoError(t, svc.ensureSubscription(ctx, "sub-b", "orders"))
	require.NoError(t, svc.ensureSubscription(ctx, "other", "shipments"))

	before := time.Now().UTC()
	resp, err := svc.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
		Topic:   "orders",
		Message: &eventv1.Message{Data: []byte("payload")}, // no publish_time -> server stamps now
	}))
	require.NoError(t, err)

	var deliveries []Delivery
	require.NoError(t, db.Order("subscription asc").Find(&deliveries).Error)
	require.Len(t, deliveries, 2, "fan-out only to subscriptions of the topic")
	require.Equal(t, "sub-a", deliveries[0].Subscription)
	require.Equal(t, "sub-b", deliveries[1].Subscription)
	seenAck := map[string]bool{}
	for _, d := range deliveries {
		require.Equal(t, resp.Msg.GetMessageId(), d.MessageID, "shared message id across fan-out copies")
		require.False(t, d.Acked)
		require.Nil(t, d.LeasedAt, "freshly published rows are not yet leased")
		require.NotEmpty(t, d.AckID)
		require.False(t, seenAck[d.AckID], "ack ids are unique per delivery")
		seenAck[d.AckID] = true
		require.False(t, d.PublishTime.IsZero(), "server must stamp publish_time when omitted")
		require.WithinDuration(t, before, d.PublishTime, time.Minute)
	}
}

func TestSend_RejectsInvalidArgs(t *testing.T) {
	t.Parallel()
	svc := NewEventService(newTestDB(t))
	ctx := context.Background()

	_, err := svc.Send(ctx, connect.NewRequest(&eventv1.SendRequest{Topic: ""}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = svc.Send(ctx, connect.NewRequest(&eventv1.SendRequest{Topic: "t", Message: nil}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestEnsureSubscription_CreatesIdempotentlyAndRequiresTopic(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	svc := NewEventService(db)
	ctx := context.Background()

	// New subscription without a topic is rejected.
	err := svc.ensureSubscription(ctx, "s1", "")
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	require.NoError(t, svc.ensureSubscription(ctx, "s1", "t1"))
	require.NoError(t, svc.ensureSubscription(ctx, "s1", "t1")) // idempotent
	require.NoError(t, svc.ensureSubscription(ctx, "s1", ""))   // existing -> topic ignored

	// Re-binding an existing subscription to a different topic is rejected
	// (immutable binding) rather than silently ignored.
	err = svc.ensureSubscription(ctx, "s1", "t2")
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	var count int64
	require.NoError(t, db.Model(&Subscription{}).Where("name = ?", "s1").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestTopicAndSubscriptionList(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	svc := NewEventService(db)
	ctx := context.Background()

	require.NoError(t, svc.ensureSubscription(ctx, "sub-z", "topic-b"))
	require.NoError(t, svc.ensureSubscription(ctx, "sub-a", "topic-a"))

	topics, err := svc.TopicList(ctx, connect.NewRequest(&eventv1.TopicListRequest{}))
	require.NoError(t, err)
	require.Equal(t, []string{"topic-a", "topic-b"}, topicNames(topics.Msg.GetTopics()))

	subs, err := svc.SubscriptionList(ctx, connect.NewRequest(&eventv1.SubscriptionListRequest{}))
	require.NoError(t, err)
	got := subs.Msg.GetSubscriptions()
	require.Len(t, got, 2)
	require.Equal(t, "sub-a", got[0].GetName())
	require.Equal(t, "topic-a", got[0].GetTopic())
	require.Equal(t, "sub-z", got[1].GetName())
}

func TestSweep_DeletesOnlyAckedExpired(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	svc := NewEventService(db)
	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)

	ackedOld := old
	ackedRecent := now
	rows := []Delivery{
		{AckID: "a1", MessageID: "m1", Topic: "t", Subscription: "s", PublishTime: now, Acked: true, AckedAt: &ackedOld, CreatedAt: old},
		{AckID: "a2", MessageID: "m2", Topic: "t", Subscription: "s", PublishTime: now, Acked: true, AckedAt: &ackedRecent, CreatedAt: now},
		{AckID: "a3", MessageID: "m3", Topic: "t", Subscription: "s", PublishTime: now, Acked: false, CreatedAt: old},
	}
	require.NoError(t, db.Create(&rows).Error)

	deleted, err := svc.sweep(context.Background(), now.Add(-1*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted, "only the acked+expired row is reclaimed")

	var remaining int64
	require.NoError(t, db.Model(&Delivery{}).Count(&remaining).Error)
	require.Equal(t, int64(2), remaining)
}

func TestPull_StreamsEventAndAck(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithPullPoll(20*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, db.Create(&Subscription{Name: "consumer", Topic: "events", CreatedAt: time.Now().UTC()}).Error)

	pub := time.Now().UTC().Add(-30 * time.Second)
	_, err := client.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
		Topic: "events",
		Message: &eventv1.Message{
			Data:        []byte("hello"),
			MessageId:   "msg-123",
			PublishTime: timestamppb.New(pub),
			Attributes:  map[string]string{"k": "v"},
		},
	}))
	require.NoError(t, err)

	stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "consumer"}))
	require.NoError(t, err)

	ev := receiveEvent(t, stream)
	require.Equal(t, "consumer", ev.GetSubscription())
	require.NotEmpty(t, ev.GetAckId())
	require.Equal(t, "msg-123", ev.GetMessage().GetMessageId())
	require.Equal(t, []byte("hello"), ev.GetMessage().GetData())
	require.Equal(t, map[string]string{"k": "v"}, ev.GetMessage().GetAttributes())
	require.WithinDuration(t, pub, ev.GetMessage().GetPublishTime().AsTime(), time.Second)

	// Acking confirms receipt and removes the row from the redelivery cycle.
	ackResp, err := client.Ack(ctx, connect.NewRequest(&eventv1.AckRequest{
		Subscription: "consumer",
		AckIds:       []string{ev.GetAckId()},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(1), ackResp.Msg.GetAcked())

	var acked int64
	require.NoError(t, db.Model(&Delivery{}).Where("subscription = ? AND acked = ?", "consumer", true).Count(&acked).Error)
	require.Equal(t, int64(1), acked)
}

func TestPull_RejectsEmptySubscriptionViaInterceptor(t *testing.T) {
	t.Parallel()
	client, _ := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: ""}))
	require.NoError(t, err) // error surfaces on receive for server streams
	require.False(t, stream.Receive())
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(stream.Err()))
}

// TestPull_DeliversToLiveStream sends WHILE a Pull stream is already open: it
// proves a message published after subscribe arrives, and that the unary Send
// is not starved by the long-poll loop holding the single SQLite connection
// (run under -race). The reader runs in a goroutine and skips the initial Ping.
func TestPull_DeliversToLiveStream(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithPullPoll(20*time.Millisecond))
	require.NoError(t, db.Create(&Subscription{Name: "live", Topic: "events", CreatedAt: time.Now().UTC()}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type recv struct {
		id  string
		err error
	}
	got := make(chan recv, 1)
	go func() {
		stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "live"}))
		if err != nil {
			got <- recv{err: err}
			return
		}
		ev, ok := nextEvent(stream)
		if !ok {
			got <- recv{err: stream.Err()}
			return
		}
		got <- recv{id: ev.GetMessage().GetMessageId()}
	}()

	_, err := client.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
		Topic:   "events",
		Message: &eventv1.Message{MessageId: "live-1", Data: []byte("x")},
	}))
	require.NoError(t, err)

	select {
	case r := <-got:
		require.NoError(t, r.err)
		require.Equal(t, "live-1", r.id)
	case <-ctx.Done():
		t.Fatal("Send was starved or message not delivered to the live stream")
	}
}

// TestPull_FIFOAndDrainsAcrossBatches pins per-subscription FIFO (Order id asc)
// and the fast-drain behavior: with pullBatch=2 and a deliberately long
// pullPoll, all 3 backlogged messages must still arrive well within one poll
// interval — which only holds if a full batch re-polls immediately instead of
// idling on the ticker.
func TestPull_FIFOAndDrainsAcrossBatches(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithPullPoll(2*time.Second), WithPullBatch(2))
	require.NoError(t, db.Create(&Subscription{Name: "fifo", Topic: "events", CreatedAt: time.Now().UTC()}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, id := range []string{"m1", "m2", "m3"} {
		_, err := client.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
			Topic:   "events",
			Message: &eventv1.Message{MessageId: id, Data: []byte(id)},
		}))
		require.NoError(t, err)
	}

	stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "fifo"}))
	require.NoError(t, err)

	start := time.Now()
	var ids []string
	for range []int{0, 1, 2} {
		ev := receiveEvent(t, stream)
		ids = append(ids, ev.GetMessage().GetMessageId())
	}
	require.Equal(t, []string{"m1", "m2", "m3"}, ids, "FIFO by insertion order")
	require.Less(t, time.Since(start), time.Second,
		"backlog spanning >1 batch must drain without waiting a full poll interval")
}

// TestPull_LazyCreatesSubscription proves the Pull RPC itself creates+binds a
// brand-new subscription (not pre-seeded) and then delivers to it.
func TestPull_LazyCreatesSubscription(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithPullPoll(20*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got := make(chan string, 1)
	go func() {
		stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "newsub", Topic: "events"}))
		if err != nil {
			return
		}
		if ev, ok := nextEvent(stream); ok {
			got <- ev.GetMessage().GetMessageId()
		}
	}()

	// Pull creates the subscription synchronously before its poll loop.
	require.Eventually(t, func() bool {
		var n int64
		require.NoError(t, db.Model(&Subscription{}).Where("name = ? AND topic = ?", "newsub", "events").Count(&n).Error)
		return n == 1
	}, 3*time.Second, 20*time.Millisecond, "Pull should have created the subscription")

	_, err := client.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
		Topic:   "events",
		Message: &eventv1.Message{MessageId: "lazy-1"},
	}))
	require.NoError(t, err)

	select {
	case id := <-got:
		require.Equal(t, "lazy-1", id)
	case <-ctx.Done():
		t.Fatal("message not delivered to lazily-created subscription")
	}
}

// TestPull_NewSubscriptionRequiresTopic exercises the handler precondition
// (distinct from the proto-validator empty-subscription path): a never-seen
// subscription with no topic cannot be bound.
func TestPull_NewSubscriptionRequiresTopic(t *testing.T) {
	t.Parallel()
	client, _ := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "brand-new", Topic: ""}))
	require.NoError(t, err)
	require.False(t, stream.Receive())
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(stream.Err()))
}

// TestMaintain_SweepsAndStreams drives the Maintain RPC end-to-end: it pins the
// retention cutoff SIGN (a +retention bug would report 0 deleted), the per-pass
// response emission, and clean shutdown on cancel.
func TestMaintain_SweepsAndStreams(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithMaintainTick(20*time.Millisecond), WithRetention(time.Hour))

	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)
	rows := []Delivery{
		{AckID: "old", MessageID: "old", Topic: "t", Subscription: "s", PublishTime: now, Acked: true, AckedAt: &old, CreatedAt: old},
		{AckID: "recent", MessageID: "recent", Topic: "t", Subscription: "s", PublishTime: now, Acked: true, AckedAt: &now, CreatedAt: now},
	}
	require.NoError(t, db.Create(&rows).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.Maintain(ctx, connect.NewRequest(&eventv1.MaintainRequest{}))
	require.NoError(t, err)
	require.True(t, stream.Receive(), "expected a maintenance tick: %v", stream.Err())
	require.GreaterOrEqual(t, stream.Msg().GetDeleted(), int64(1), "the expired delivered row should be reclaimed")

	var remaining int64
	require.NoError(t, db.Model(&Delivery{}).Count(&remaining).Error)
	require.Equal(t, int64(1), remaining, "the recent delivered row must survive")

	cancel()
	require.False(t, stream.Receive(), "stream should end after cancel")
}

// TestPull_RedeliversUnackedAfterDeadline is the at-least-once guarantee: a
// message that is delivered but never acked is re-served once its lease passes
// the ack deadline (the consumer-crash path).
func TestPull_RedeliversUnackedAfterDeadline(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithPullPoll(20*time.Millisecond), WithAckDeadline(150*time.Millisecond))
	require.NoError(t, db.Create(&Subscription{Name: "redel", Topic: "events", CreatedAt: time.Now().UTC()}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
		Topic:   "events",
		Message: &eventv1.Message{MessageId: "redel-1"},
	}))
	require.NoError(t, err)

	stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "redel"}))
	require.NoError(t, err)

	first := receiveEvent(t, stream)
	require.Equal(t, "redel-1", first.GetMessage().GetMessageId())

	// Never acked -> redelivered after the ack deadline, reusing the ack id.
	second := receiveEvent(t, stream)
	require.Equal(t, "redel-1", second.GetMessage().GetMessageId())
	require.Equal(t, first.GetAckId(), second.GetAckId(), "redelivery reuses the same ack id")
}

// TestAck_StopsRedelivery confirms an acked message is not re-served even after
// the ack deadline elapses.
func TestAck_StopsRedelivery(t *testing.T) {
	t.Parallel()
	client, db := newTestServer(t, WithPullPoll(20*time.Millisecond), WithAckDeadline(100*time.Millisecond))
	require.NoError(t, db.Create(&Subscription{Name: "acksub", Topic: "events", CreatedAt: time.Now().UTC()}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Send(ctx, connect.NewRequest(&eventv1.SendRequest{
		Topic:   "events",
		Message: &eventv1.Message{MessageId: "ack-1"},
	}))
	require.NoError(t, err)

	stream, err := client.Pull(ctx, connect.NewRequest(&eventv1.PullRequest{Subscription: "acksub"}))
	require.NoError(t, err)

	ev := receiveEvent(t, stream)
	ackResp, err := client.Ack(ctx, connect.NewRequest(&eventv1.AckRequest{
		Subscription: "acksub",
		AckIds:       []string{ev.GetAckId()},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(1), ackResp.Msg.GetAcked())

	// No redelivery on the same stream across several ack deadlines.
	redelivered := make(chan string, 1)
	go func() {
		if ev2, ok := nextEvent(stream); ok {
			redelivered <- ev2.GetMessage().GetMessageId()
		}
	}()
	select {
	case id := <-redelivered:
		t.Fatalf("acked message must not be redelivered, got %q", id)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestAck_RejectsInvalidArgs(t *testing.T) {
	t.Parallel()
	svc := NewEventService(newTestDB(t))
	ctx := context.Background()

	_, err := svc.Ack(ctx, connect.NewRequest(&eventv1.AckRequest{Subscription: "", AckIds: []string{"x"}}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = svc.Ack(ctx, connect.NewRequest(&eventv1.AckRequest{Subscription: "s", AckIds: nil}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestAck_IgnoresForeignAckIds checks Ack only touches the caller's own
// subscription and reports the real count (unknown/foreign ids are ignored).
func TestAck_IgnoresForeignAckIds(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	svc := NewEventService(db)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&[]Delivery{
		{AckID: "mine", MessageID: "m", Topic: "t", Subscription: "s1", PublishTime: now, CreatedAt: now},
		{AckID: "other", MessageID: "m", Topic: "t", Subscription: "s2", PublishTime: now, CreatedAt: now},
	}).Error)

	resp, err := svc.Ack(context.Background(), connect.NewRequest(&eventv1.AckRequest{
		Subscription: "s1",
		AckIds:       []string{"mine", "other", "does-not-exist"},
	}))
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Msg.GetAcked(), "only s1's own ack id is acked")

	var s2Acked int64
	require.NoError(t, db.Model(&Delivery{}).Where("ack_id = ? AND acked = ?", "other", true).Count(&s2Acked).Error)
	require.Equal(t, int64(0), s2Acked, "another subscription's delivery is untouched")
}

func topicNames(ts []*eventv1.Topic) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.GetName())
	}
	return out
}

// nextEvent reads from a Pull stream, skipping Ping (heartbeat/header-flush)
// frames, and returns the first Event. ok is false when the stream ends first.
// Safe to call from a goroutine (it never touches *testing.T).
func nextEvent(stream *connect.ServerStreamForClient[eventv1.PullResponse]) (*eventv1.Event, bool) {
	for stream.Receive() {
		if ev := stream.Msg().GetEvent(); ev != nil {
			return ev, true
		}
	}
	return nil, false
}

// receiveEvent is the test-goroutine variant of nextEvent that fails the test
// if the stream ends before an Event arrives.
func receiveEvent(t *testing.T, stream *connect.ServerStreamForClient[eventv1.PullResponse]) *eventv1.Event {
	t.Helper()
	ev, ok := nextEvent(stream)
	if !ok {
		t.Fatalf("stream ended before an event arrived: %v", stream.Err())
	}
	return ev
}
