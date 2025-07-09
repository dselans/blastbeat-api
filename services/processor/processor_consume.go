package processor

import (
	"context"
	"runtime/debug"

	"github.com/newrelic/go-agent/v3/newrelic"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/superpowerdotcom/events/build/proto/go/common"
	"github.com/superpowerdotcom/go-lib-common/util"
	"github.com/superpowerdotcom/go-lib-common/validate"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// ConsumeFunc is a consumer function that will be executed by the "rabbit"
// library whenever Consume() rads a new message from RabbitMQ.
func (p *Processor) ConsumeFunc(msg amqp.Delivery) error {
	logger := p.log.With(
		zap.String("method", "ConsumeFunc"),
		zap.String("routingKey", msg.RoutingKey),
	)

	txn := p.options.NewRelic.StartTransaction("ProcessorService.ConsumeFunc")
	defer txn.End()

	// ConsumeFunc runs in goroutine
	defer func() {
		if r := recover(); r != nil {
			util.Error(txn, logger, "recovered from panic", nil,
				zap.Any("panic", r),
				zap.Stack("stack"),
				zap.Any("panicTrace", string(debug.Stack())),
			)
		}
	}()

	// logger.Debug("Received (unvalidated) message on event bus")

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
		util.Error(txn, logger, "unable to acknowledge message", err)
		return nil
	}

	// Try to decode message and dispatch it accordingly
	event := &common.Event{}

	if err := proto.Unmarshal(msg.Body, event); err != nil {
		util.Error(txn, logger, "unable to unmarshal event", err)
		return nil
	}

	if err := validate.Event(event); err != nil {
		util.Error(txn, logger, "unable to validate event", err)
		return nil
	}

	logger = logger.With(
		zap.String("cloudEventID", event.Id),
		zap.String("cloudEventType", event.Type),
		zap.String("cloudEventSource", event.Source),
	)

	// Create context with logger that we can pass around
	ctx := context.WithValue(context.Background(), "logger", logger)

	// Now add NewRelic txn to context
	ctx = newrelic.NewContext(ctx, txn)

	// Add cloud events attributes to NewRelic txn
	txn.AddAttribute("cloudEventID", event.Id)
	txn.AddAttribute("cloudEventType", event.Type)
	txn.AddAttribute("cloudEventSource", event.Source)

	// logger.Debug("Validated event message")

	var err error

	switch event.Data.(type) {
	case *common.Event_MedplumWebhook:
		err = p.handleMedplumWebhook(ctx, event)
	default:
		// logger.Debug("Unknown message type", zap.String("type", event.Type))
		return nil
	}

	if err != nil {
		util.Error(txn, logger, "error processing message", err)
		return nil
	}

	return nil
}
