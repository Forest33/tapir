package ack

import (
	"context"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

type Ack struct {
	ctx       context.Context
	cancel    context.CancelFunc
	log       *logger.Logger
	cfg       *Config
	sender    entity.AckSender
	conn      *entity.Connection
	sessionID uint32
	senderCh  chan message
	partOfRTO float64
}

type Config struct {
	MaxSize                 int
	EndpointLifeTime        int64
	WaitingTimePercentOfRTO float64
	Tracing                 bool
}

type message struct {
	id       uint32
	endpoint entity.PacketEndpoint
}

func New(ctx context.Context, log *logger.Logger, cfg *Config, sender entity.AckSender, conn *entity.Connection, sessionID uint32) *Ack {
	a := &Ack{
		log:       log.Duplicate(log.With().Str("layer", "ack").Logger()),
		cfg:       cfg,
		sender:    sender,
		conn:      conn,
		sessionID: sessionID,
		senderCh:  make(chan message, cfg.MaxSize),
		partOfRTO: cfg.WaitingTimePercentOfRTO / 100.0,
	}

	a.ctx, a.cancel = context.WithCancel(ctx)

	go a.accumulator()

	return a
}

func (a *Ack) Push(msgID uint32, endpoint entity.PacketEndpoint) {
	a.senderCh <- message{
		id:       msgID,
		endpoint: endpoint,
	}
}

func (a *Ack) Stop() {
	a.cancel()
}

func (a *Ack) accumulator() {
	var (
		ackIDs       = entity.NewMessageAcknowledgement(nil).SetMaxSize(a.cfg.MaxSize)
		firstAckTime int64
		added        bool
	)

	send := func() {
		a.sendACK(ackIDs)
		ackIDs.Reset()
		firstAckTime = 0
	}

	for {
		select {
		case msg := <-a.senderCh:
			if firstAckTime == 0 {
				firstAckTime = time.Now().UnixNano()
			}
			added = ackIDs.Push(msg.endpoint, msg.id)
			if time.Now().UnixNano()-firstAckTime >= int64(float64(a.conn.Retry.GetRTO().Nanoseconds())*a.partOfRTO) || !added {
				send()
				if !added {
					ackIDs.Push(msg.endpoint, msg.id)
				}
			}
		case <-time.After(time.Duration(float64(a.conn.Retry.GetRTO().Nanoseconds()) * a.partOfRTO)):
			send()
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *Ack) sendACK(ackIDs *entity.MessageAcknowledgement) {
	if ackIDs.Size() == 0 {
		return
	}

	ack := &entity.Message{
		SessionID: a.sessionID,
		Type:      entity.MessageTypeData,
		IsACK:     true,
		Payload:   ackIDs,
	}
	if err := a.sender(ack, nil, a.conn, nil); err != nil {
		a.log.Error().Err(err).
			Str("type", ack.Type.String()).
			Uint32("id", ack.ID).
			Msg("failed to send acknowledgement")
	}

	if a.cfg.Tracing {
		a.log.Debug().
			Int("ack_size", ackIDs.GetMessagesCount()).
			Interface("ack_id", ackIDs.Get()).
			Float64("rto", a.conn.Retry.GetRTO().Seconds()).
			Msg("sending acknowledgement")
	}
}
