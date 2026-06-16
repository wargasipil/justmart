package warga_event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	eventv1 "github.com/wargasipil/warga_collections/definitions/event_iface/v1"
	"github.com/wargasipil/warga_collections/definitions/event_iface/v1/event_ifacev1connect"
)

const (
	defaultPullPoll     = 250 * time.Millisecond
	defaultPullBatch    = 100
	defaultAckDeadline  = 30 * time.Second
	defaultHeartbeat    = 30 * time.Second
	defaultMaintainTick = 30 * time.Second
	defaultRetention    = 24 * time.Hour
)

// eventServiceImpl is a SQLite/GORM-backed pub/sub broker implementing the
// EventService handler.
//
// Delivery is at-least-once with explicit acknowledgement. Pull leases a
// message (streams it as an Event carrying an opaque ack_id and records
// LeasedAt) and the consumer confirms receipt via Ack. An unacked message is
// redelivered once its lease is older than the ack deadline — so a consumer
// crash or a connection drop after send results in redelivery, not loss.
// Maintain reclaims acked rows past the retention window.
//
// There is no per-row exclusive claim, so two concurrent Pull streams on the
// same subscription can still deliver the same message within the ack-deadline
// window; intended usage is one consumer per subscription, and consumers must
// tolerate duplicates. A subscription with no live consumer retains its
// unacked rows indefinitely; delete such subscriptions out of band to bound
// storage.
type eventServiceImpl struct {
	db *gorm.DB

	pullPoll     time.Duration
	pullBatch    int
	ackDeadline  time.Duration
	heartbeat    time.Duration
	maintainTick time.Duration
	retention    time.Duration
}

// Option customizes broker timing/retention.
type Option func(*eventServiceImpl)

// WithPullPoll sets how often an idle Pull stream polls for new messages.
func WithPullPoll(d time.Duration) Option {
	return func(s *eventServiceImpl) {
		if d > 0 {
			s.pullPoll = d
		}
	}
}

// WithPullBatch sets the maximum number of messages read per poll.
func WithPullBatch(n int) Option {
	return func(s *eventServiceImpl) {
		if n > 0 {
			s.pullBatch = n
		}
	}
}

// WithAckDeadline sets how long a leased (sent but unacked) message waits before
// it becomes eligible for redelivery.
func WithAckDeadline(d time.Duration) Option {
	return func(s *eventServiceImpl) {
		if d > 0 {
			s.ackDeadline = d
		}
	}
}

// WithHeartbeat sets how long an idle Pull stream waits before sending a Ping
// frame (which also flushes the response headers on connect).
func WithHeartbeat(d time.Duration) Option {
	return func(s *eventServiceImpl) {
		if d > 0 {
			s.heartbeat = d
		}
	}
}

// WithMaintainTick sets how often a Maintain stream runs a sweep.
func WithMaintainTick(d time.Duration) Option {
	return func(s *eventServiceImpl) {
		if d > 0 {
			s.maintainTick = d
		}
	}
}

// WithRetention sets how long a delivered message is kept before Maintain
// reclaims it.
func WithRetention(d time.Duration) Option {
	return func(s *eventServiceImpl) {
		if d > 0 {
			s.retention = d
		}
	}
}

// NewEventService builds the broker over an already-migrated *gorm.DB.
func NewEventService(db *gorm.DB, opts ...Option) *eventServiceImpl {
	s := &eventServiceImpl{
		db:           db,
		pullPoll:     defaultPullPoll,
		pullBatch:    defaultPullBatch,
		ackDeadline:  defaultAckDeadline,
		heartbeat:    defaultHeartbeat,
		maintainTick: defaultMaintainTick,
		retention:    defaultRetention,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

var _ event_ifacev1connect.EventServiceHandler = (*eventServiceImpl)(nil)

// Send publishes a message to a topic, fanning it out to every subscription
// currently bound to that topic. A topic with no subscriptions still records
// the topic; the message is dropped (no consumer to deliver it to).
func (s *eventServiceImpl) Send(ctx context.Context, req *connect.Request[eventv1.SendRequest]) (*connect.Response[eventv1.SendResponse], error) {
	topic := req.Msg.GetTopic()
	if topic == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("topic is required"))
	}
	msg := req.Msg.GetMessage()
	if msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("message is required"))
	}

	messageID := msg.GetMessageId()
	if messageID == "" {
		messageID = uuid.NewString()
	}
	publish := time.Now().UTC()
	if pt := msg.GetPublishTime(); pt != nil && pt.IsValid() {
		publish = pt.AsTime().UTC()
	}
	attrs, err := json.Marshal(msg.GetAttributes())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertTopic(tx, topic, now); err != nil {
			return err
		}
		var subs []Subscription
		if err := tx.Where("topic = ?", topic).Find(&subs).Error; err != nil {
			return err
		}
		if len(subs) == 0 {
			return nil
		}
		deliveries := make([]Delivery, 0, len(subs))
		for _, sub := range subs {
			deliveries = append(deliveries, Delivery{
				AckID:        uuid.NewString(),
				MessageID:    messageID,
				Topic:        topic,
				Subscription: sub.Name,
				Data:         msg.GetData(),
				Attributes:   string(attrs),
				PublishTime:  publish,
				CreatedAt:    now,
			})
		}
		return tx.Create(&deliveries).Error
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&eventv1.SendResponse{MessageId: messageID}), nil
}

// Pull binds (creating if necessary) the subscription and streams its messages
// as Events, leasing each (recording LeasedAt) after a successful send so it is
// not re-sent until the ack deadline lapses. The consumer confirms receipt via
// Ack; unacked messages are redelivered after the deadline. An initial Ping
// flushes the response headers immediately (so the client's Pull call returns
// even on an idle subscription), and further Pings keep an idle stream alive.
// The stream runs until the client disconnects or the context is cancelled.
func (s *eventServiceImpl) Pull(ctx context.Context, req *connect.Request[eventv1.PullRequest], stream *connect.ServerStream[eventv1.PullResponse]) error {
	sub := req.Msg.GetSubscription()
	if sub == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("subscription is required"))
	}
	if err := s.ensureSubscription(ctx, sub, req.Msg.GetTopic()); err != nil {
		return err
	}

	// Flush headers right away so client.Pull returns without waiting for the
	// first message, and so idle long-polls aren't dropped by proxies.
	if err := stream.Send(&eventv1.PullResponse{D: &eventv1.PullResponse_Ping{Ping: &eventv1.Ping{}}}); err != nil {
		return err
	}
	lastFrame := time.Now()

	ticker := time.NewTicker(s.pullPoll)
	defer ticker.Stop()

	for {
		cutoff := time.Now().UTC().Add(-s.ackDeadline)
		var rows []Delivery
		if err := s.db.WithContext(ctx).
			Where("subscription = ? AND acked = ? AND (leased_at IS NULL OR leased_at < ?)", sub, false, cutoff).
			Order("id asc").
			Limit(s.pullBatch).
			Find(&rows).Error; err != nil {
			if ctxDone(ctx, err) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, err)
		}

		for i := range rows {
			d := &rows[i]
			pm, err := toProtoMessage(d)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			event := &eventv1.Event{Subscription: sub, Message: pm, AckId: d.AckID}
			if err := stream.Send(&eventv1.PullResponse{D: &eventv1.PullResponse_Event{Event: event}}); err != nil {
				return err
			}
			lastFrame = time.Now()
			now := lastFrame.UTC()
			if err := s.db.WithContext(ctx).
				Model(&Delivery{}).
				Where("id = ?", d.ID).
				Update("leased_at", now).Error; err != nil {
				if ctxDone(ctx, err) {
					return nil
				}
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		// A full batch means more rows are likely pending: drain at DB/consumer
		// speed and only fall back to the idle poll interval once caught up
		// (otherwise a backlog would drain at just pullBatch/pullPoll msgs/sec).
		if len(rows) == s.pullBatch {
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if time.Since(lastFrame) >= s.heartbeat {
				if err := stream.Send(&eventv1.PullResponse{D: &eventv1.PullResponse_Ping{Ping: &eventv1.Ping{}}}); err != nil {
					return err
				}
				lastFrame = time.Now()
			}
		}
	}
}

// Ack acknowledges leased messages by their opaque ack ids, removing them from
// the redelivery cycle. Unknown or foreign ack ids are ignored; the response
// reports how many rows were actually acked.
func (s *eventServiceImpl) Ack(ctx context.Context, req *connect.Request[eventv1.AckRequest]) (*connect.Response[eventv1.AckResponse], error) {
	sub := req.Msg.GetSubscription()
	if sub == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("subscription is required"))
	}
	ackIDs := req.Msg.GetAckIds()
	if len(ackIDs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one ack_id is required"))
	}

	now := time.Now().UTC()
	res := s.db.WithContext(ctx).
		Model(&Delivery{}).
		Where("subscription = ? AND acked = ? AND ack_id IN ?", sub, false, ackIDs).
		Updates(map[string]any{"acked": true, "acked_at": now})
	if res.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	return connect.NewResponse(&eventv1.AckResponse{Acked: res.RowsAffected}), nil
}

// TopicList returns all known topics ordered by name.
func (s *eventServiceImpl) TopicList(ctx context.Context, _ *connect.Request[eventv1.TopicListRequest]) (*connect.Response[eventv1.TopicListResponse], error) {
	var topics []Topic
	if err := s.db.WithContext(ctx).Order("name asc").Find(&topics).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*eventv1.Topic, 0, len(topics))
	for _, t := range topics {
		out = append(out, &eventv1.Topic{
			Name:       t.Name,
			CreateTime: timestamppb.New(t.CreatedAt),
		})
	}
	return connect.NewResponse(&eventv1.TopicListResponse{Topics: out}), nil
}

// SubscriptionList returns all known subscriptions ordered by name.
func (s *eventServiceImpl) SubscriptionList(ctx context.Context, _ *connect.Request[eventv1.SubscriptionListRequest]) (*connect.Response[eventv1.SubscriptionListResponse], error) {
	var subs []Subscription
	if err := s.db.WithContext(ctx).Order("name asc").Find(&subs).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*eventv1.Subscription, 0, len(subs))
	for _, sub := range subs {
		out = append(out, &eventv1.Subscription{
			Name:       sub.Name,
			Topic:      sub.Topic,
			CreateTime: timestamppb.New(sub.CreatedAt),
		})
	}
	return connect.NewResponse(&eventv1.SubscriptionListResponse{Subscriptions: out}), nil
}

// Maintain runs a periodic sweep that reclaims acknowledged messages older than
// the retention window, emitting a count after each pass. Runs until the client
// disconnects or the context is cancelled.
func (s *eventServiceImpl) Maintain(ctx context.Context, _ *connect.Request[eventv1.MaintainRequest], stream *connect.ServerStream[eventv1.MaintainResponse]) error {
	ticker := time.NewTicker(s.maintainTick)
	defer ticker.Stop()

	for {
		deleted, err := s.sweep(ctx, time.Now().UTC().Add(-s.retention))
		if err != nil {
			if ctxDone(ctx, err) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := stream.Send(&eventv1.MaintainResponse{
			Deleted: deleted,
			Time:    timestamppb.New(time.Now().UTC()),
		}); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// sweep deletes acknowledged messages whose AckedAt is older than cutoff and
// returns how many rows were removed.
func (s *eventServiceImpl) sweep(ctx context.Context, cutoff time.Time) (int64, error) {
	res := s.db.WithContext(ctx).
		Where("acked = ? AND acked_at IS NOT NULL AND acked_at < ?", true, cutoff).
		Delete(&Delivery{})
	return res.RowsAffected, res.Error
}

// ensureSubscription creates the subscription (and its topic) if it does not
// exist. topic is required only when the subscription is new.
func (s *eventServiceImpl) ensureSubscription(ctx context.Context, name, topic string) error {
	var existing Subscription
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&existing).Error
	if err == nil {
		// A subscription's topic binding is immutable. Silently ignoring a
		// contradictory topic would leave the caller long-polling a different
		// topic than it asked for (a silent dead consumer), so fail loudly.
		if topic != "" && topic != existing.Topic {
			return connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("subscription %q is bound to topic %q, not %q", name, existing.Topic, topic))
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		if ctxDone(ctx, err) {
			return connect.NewError(connect.CodeCanceled, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if topic == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("topic is required to create subscription "+name))
	}

	now := time.Now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertTopic(tx, topic, now); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&Subscription{Name: name, Topic: topic, CreatedAt: now}).Error
	})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

func upsertTopic(tx *gorm.DB, name string, now time.Time) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&Topic{Name: name, CreatedAt: now}).Error
}

func toProtoMessage(d *Delivery) (*eventv1.Message, error) {
	attrs := map[string]string{}
	if d.Attributes != "" {
		if err := json.Unmarshal([]byte(d.Attributes), &attrs); err != nil {
			return nil, err
		}
	}
	if len(attrs) == 0 {
		attrs = nil
	}
	return &eventv1.Message{
		Data:        d.Data,
		MessageId:   d.MessageID,
		PublishTime: timestamppb.New(d.PublishTime),
		Attributes:  attrs,
	}, nil
}

// ctxDone reports whether err is the result of ctx being cancelled or timed
// out — used to turn a stream into a clean close instead of an Internal error.
func ctxDone(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
