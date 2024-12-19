package processor

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/go-lib-common/validate"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// ConsumeFunc is a consumer function that will be executed by the "rabbit"
// library whenever Consume() rads a new message from RabbitMQ.
func (p *Processor) ConsumeFunc(msg amqp.Delivery) error {
	logger := p.log.With(zap.String("method", "ConsumeFunc"))

	// ConsumeFunc runs in goroutine
	defer func() {
		if r := recover(); r != nil {
			logger.Error("recovered from panic", zap.Any("recovered", r))
		}
	}()

	txn := p.options.NewRelic.StartTransaction("ConsumeFunc")
	defer txn.End()

	logger = logger.With(zap.String("routingKey", msg.RoutingKey))

	logger.Debug("Received (unvalidated) message on event bus")

	// !!!!
	//
	// You should leave this as-is during initial dev as it'll simplify not
	// having to worry about re-queueing logic. Once you're ready for prod,
	// you should probably remove this and *properly* handle ACKs/NACKs (
	// (ie. ACK only when actually process, NACK w/ requeue on non-fatal error,
	// NACK w/o requeue on fatal error).
	//
	// !!!!
	if err := msg.Ack(false); err != nil {
		logger.Error("Error acknowledging message", zap.Error(err))
		return nil
	}

	// Try to decode message and dispatch it accordingly
	event := &common.Event{}

	if err := proto.Unmarshal(msg.Body, event); err != nil {
		// TODO: Record error in NR
		logger.Error("Error unmarshalling event", zap.Error(err))
		return nil
	}

	if err := validate.Event(event); err != nil {
		// TODO: Record error in NR
		logger.Error("Error validating event", zap.Error(err))
		return nil
	}

	logger = logger.With(
		zap.String("id", event.Id),
		zap.String("type", event.Type),
		zap.String("source", event.Source),
	)

	logger.Debug("Validated event message")

	var err error

	switch event.Data.(type) {
	case *common.Event_UserCreated:
		err = p.handleUserCreated(event)
	default:
		logger.Error("Unknown message type", zap.String("type", event.Type))
		return nil
	}

	if err != nil {
		// TODO: Record error in NR
		logger.Error("Error processing message", zap.Error(err))
		return nil
	}

	return nil
}
