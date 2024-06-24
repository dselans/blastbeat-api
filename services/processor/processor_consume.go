package processor

import (
	"net"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	DialTimeout = 5 * time.Second
)

// ConsumeFunc is a consumer function that will be executed by the "rabbit"
// library whenever Consume() rads a new message from RabbitMQ.
func (p *Processor) ConsumeFunc(msg amqp.Delivery) error {
	logger := p.log.With(zap.String("method", "ConsumeFunc"))

	// ConsumeFunc runs in goroutuine
	defer func() {
		if r := recover(); r != nil {
			logger.Error("recovered from panic", zap.Any("recovered", r))
		}
	}()

	txn := p.options.NewRelic.StartTransaction("ConsumeFunc")
	defer txn.End()

	// logger.Debug("Received message: " + string(msg.Body))

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

	// Do something with the delivered message

	return nil
}

func newClient() *http.Client {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: DialTimeout,
			}).DialContext,
		},
	}

	return &client
}
