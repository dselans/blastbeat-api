package processor

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func (p *Processor) ReplayFunc(msg amqp.Delivery) error {
	p.log.Debug("Handling message in replayer", zap.String("routingKey", msg.RoutingKey))
	if err := p.ConsumeFunc(msg); err != nil {
		p.log.Error("Error consuming message in replayer", zap.Error(err))
		return err
	}

	return nil
}
