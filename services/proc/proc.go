// Package proc is used for acting on messages received from RabbitMQ
package proc

import (
	"context"
	"fmt"
	"reflect"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/streamdal/rabbit"
	"go.uber.org/zap"

	"github.com/streamdal/go-svc-template/backends/cache"
	"github.com/streamdal/go-svc-template/clog"
	"github.com/streamdal/go-svc-template/config"
)

const (
	DefaultNumConsumers = 10
)

type IProc interface {
	StartConsumers() error
}

type Options struct {
	RabbitMap map[string]*RabbitConfig
	Cache     cache.ICache
	Log       clog.ICustomLog
	NewRelic  *newrelic.Application
}

type RabbitConfig struct {
	RabbitInstance rabbit.IRabbit
	NumConsumers   int
	Func           string
	funcReal       func(amqp.Delivery) error // filled out during New()
}

type Proc struct {
	config  *config.Config
	options *Options
	log     clog.ICustomLog
}

func New(opt *Options, cfg *config.Config) (*Proc, error) {
	if opt == nil {
		return nil, errors.New("options cannot be nil")
	}

	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	// We have to instantiate this because validateOptions needs access to our instance
	i := &Proc{
		config:  cfg,
		options: opt,
	}

	if err := i.validateOptions(opt); err != nil {
		return nil, fmt.Errorf("unable to validate input opt: %s", err)
	}

	i.log = opt.Log.With(zap.String("pkg", "proc"))

	return i, nil
}

func (p *Proc) validateOptions(opts *Options) error {
	if opts.Cache == nil {
		return errors.New("CacheBackend cannot be nil")
	}

	if opts.Log == nil {
		return errors.New("Log cannot be nil")
	}

	if len(opts.RabbitMap) == 0 {
		return errors.New("Rabbit map cannot be empty")
	}

	for name, c := range opts.RabbitMap {
		if c.RabbitInstance == nil {
			return fmt.Errorf("rabbit instance for '%p' cannot be nil", name)
		}

		if c.Func == "" {
			return fmt.Errorf("func for '%p' cannot be nil", name)
		}

		if c.NumConsumers < 1 {
			c.NumConsumers = DefaultNumConsumers
		}

		// Is this a legit legit function?
		method := reflect.ValueOf(p).MethodByName(c.Func)

		if !method.IsValid() {
			return fmt.Errorf("method for '%p' appears to be invalid", c.Func)
		}

		f, ok := method.Interface().(func(amqp.Delivery) error)
		if !ok {
			return fmt.Errorf("unable to type assert method '%p'", c.Func)
		}

		opts.RabbitMap[name].funcReal = f
	}

	return nil
}

func (p *Proc) StartConsumers() error {
	logger := p.log.With(zap.String("method", "StartConsumers"))
	consumerErrCh := make(chan *rabbit.ConsumeError, 1)

	go p.runConsumerErrorWatcher(consumerErrCh)

	for name, r := range p.options.RabbitMap {
		logger.Debug("Launching proc consumers", zap.Int("numConsumers", r.NumConsumers), zap.String("entryName", name))

		for n := 0; n < r.NumConsumers; n++ {
			go r.RabbitInstance.Consume(context.Background(), consumerErrCh, r.funcReal)
		}
	}

	return nil
}

func (p *Proc) runConsumerErrorWatcher(errCh chan *rabbit.ConsumeError) {
	logger := p.log.With(zap.String("method", "runConsumerErrorWatcher"))

	logger.Debug("Starting")
	defer logger.Debug("Exiting")

	for {
		select {
		case err := <-errCh:
			msgID := "unknown"
			consumerTag := "unknown"

			if err.Message != nil {
				msgID = err.Message.MessageId
				consumerTag = err.Message.ConsumerTag
			}

			logger.Error("Received error from consumer",
				zap.String("error", err.Error.Error()),
				zap.String("messageId", msgID),
				zap.String("consumerTag", consumerTag),
			)
		}
	}
}
