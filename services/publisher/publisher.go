// Package publisher is a service-level wrapper library for predictably and
// reliably publishing messages to RabbitMQ. After instantiation, you must call
// Start() to start a publisher pool. Once that is done, you can call Publish()
// and the library will use the local worker pool to publish messages to Rabbit.
//
// NOTE: This package supports graceful shutdown (with timeouts). Once a
// graceful shutdown has occurred, you MUST discard the publisher instance and
// re-create it.
//
// If additional performance is needed, you have a few options:
//
//  1. Increase number of workers in pool (by tweaking PublisherNumWorkers)
//  2. Increase buffer size of the work channel
//  3. Add a batch writer that will batch messages together before publishing.
//     Something that might work - batch up to 100 messages OR if 1s has passed -
//     whichever one occurs first.
//  4. Compress messages before sending them to RabbitMQ.
package publisher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/streamdal/rabbit"
	"github.com/superpowerdotcom/events/build/proto/go/user"
	"go.uber.org/zap"

	"github.com/superpowerdotcom/go-common-lib/clog"
)

const (
	CloudEventsSpecVersion     = "1.0"
	CloudEventsDataContentType = "application/protobuf"
	CloudEventsSource          = "go-svc-template"

	DefaultNumWorkers        = 10
	PublishRequestBufferSize = 1000
	WorkerShutdownTimeout    = 5 * time.Second
)

type IPublisher interface {
	Start() error
	Stop() error

	// Publish is a "raw" method for publishing data to RabbitMQ. It does not
	// perform any additional encoding on the data and will write it as-is.
	//
	// NOTE: This method is not recommended for general use. Instead, use one of
	// the more specific Publish* methods that will handle event generation and
	// encoding for you.
	Publish(ctx context.Context, data []byte, routingKey string) error

	// PublishUserCreatedEvent generates a user.Created protobuf event and
	// publishes it to event bus 'events:user.Created'.
	PublishUserCreatedEvent(ctx context.Context, newUser *user.User) error
}

type Publisher struct {
	started                bool
	startedMtx             *sync.RWMutex
	workCh                 chan *PublishRequest
	workerWg               *sync.WaitGroup
	internalShutdownCtx    context.Context
	internalShutdownCancel context.CancelFunc
	options                *Options
	log                    clog.ICustomLog
}

type Options struct {
	RabbitBackend rabbit.IRabbit
	NumWorkers    int

	// Global context used by service to indicate that a shutdown needs to occur
	ExternalShutdownCtx context.Context

	// Channel used by main() to indicate that publisher has completed shutdown.
	// Read by main(), written to by publisher.
	ExternalShutdownDoneCh chan<- struct{}

	NewRelic *newrelic.Application
	Log      clog.ICustomLog
}

type PublishRequest struct {
	Data       []byte
	RoutingKey string
}

func New(opts *Options) (*Publisher, error) {
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, "failed to validate options")
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Publisher{
		startedMtx:             &sync.RWMutex{},
		workCh:                 make(chan *PublishRequest, PublishRequestBufferSize),
		workerWg:               &sync.WaitGroup{},
		internalShutdownCtx:    ctx,
		internalShutdownCancel: cancel,
		log:                    opts.Log.With(zap.String("pkg", "publisher")),
		options:                opts,
	}

	// Run goroutine that will ping external shutdown done channel whenever
	// publisher component is done shutting down.
	go p.runExternalShutdownListener()

	return p, nil
}

func validateOptions(opts *Options) error {
	if opts == nil {
		return errors.New("options cannot be nil")
	}

	if opts.RabbitBackend == nil {
		return errors.New("rabbit backend cannot be nil")
	}

	if opts.Log == nil {
		return errors.New("log cannot be nil")
	}

	if opts.NumWorkers == 0 {
		opts.NumWorkers = DefaultNumWorkers
	}

	if opts.ExternalShutdownCtx == nil {
		return errors.New("external shutdown context cannot be nil")
	}

	if opts.ExternalShutdownDoneCh == nil {
		return errors.New("external shutdown done channel cannot be nil")
	}

	return nil
}

func (p *Publisher) Publish(ctx context.Context, data []byte, routingKey string) error {
	if !p.isStarted() {
		return errors.New("publisher not started")
	}

	// We are probably being called from an HTTP handler which already has NR txn
	txn := newrelic.FromContext(ctx)

	segment := txn.StartSegment("publish_channel")
	defer segment.End()

	// Try to write to publish channel
	select {
	case <-p.options.ExternalShutdownCtx.Done():
		return errors.New("detected external shutdown - publish aborted")
	case <-p.internalShutdownCtx.Done():
		return errors.New("internal shutdown detected - publish aborted")
	case <-ctx.Done():
		return errors.New("context cancelled - publish aborted")
	case p.workCh <- &PublishRequest{
		Data:       data,
		RoutingKey: routingKey,
	}:
		// Successfully wrote to channel
	}

	return nil
}

func (p *Publisher) run(id int) error {
	llog := p.log.With(zap.String("method", "run"), zap.Int("id", id))
	llog.Debug("worker start")
	defer llog.Debug("worker exit")

MAIN:
	for {
		select {
		case <-p.options.ExternalShutdownCtx.Done():
			llog.Debug("external shutdown detected - exiting")
			break MAIN
		case <-p.internalShutdownCtx.Done():
			llog.Debug("internal shutdown detected - exiting")
			break MAIN
		case req, ok := <-p.workCh:
			if !ok {
				llog.Debug("work channel closed - exiting")
				break MAIN
			}

			txn := p.options.NewRelic.StartTransaction("publish_rabbit")

			if err := p.options.RabbitBackend.Publish(p.internalShutdownCtx, req.RoutingKey, req.Data); err != nil {
				llog.Error("failed to publish message", zap.Error(err))
				txn.NoticeError(errors.Wrap(err, "failed to publish message"))
			}

			txn.End()
		}
	}

	return nil
}

// Start starts up the publisher worker group
func (p *Publisher) Start() error {
	if p.isStarted() {
		return errors.New("publisher already started")
	}

	p.setStarted(true)

	errCh := make(chan error, p.options.NumWorkers)

	for i := 0; i < p.options.NumWorkers; i++ {
		go func() {
			p.workerWg.Add(1)
			defer p.workerWg.Done()

			if err := p.run(i); err != nil {
				errCh <- err
			}
		}()
	}

	// Wait for 5s for startup to complete without errors
	select {
	case <-time.After(5 * time.Second):
		// Successful startup
		return nil
	case err := <-errCh:
		p.log.Error("worker returned error during startup", zap.Error(err))

		// Shutdown remaining workers
		if err := p.Stop(); err != nil {
			p.log.Error("failed to Stop() worker group", zap.Error(err))
		}

		return err
	}
}

func (p *Publisher) runExternalShutdownListener() {
	// Listen for external shutdown signal
	<-p.options.ExternalShutdownCtx.Done()

	// Ask all workers to shutdown (if they haven't already)
	if err := p.Stop(); err != nil {
		p.log.Error("failed to stop publisher", zap.String("method", "runExternalShutdownListener"), zap.Error(err))
	}

	// Wait for workers to exit
	p.workerWg.Wait()

	// Let caller know that publisher component has completed shutdown
	p.options.ExternalShutdownDoneCh <- struct{}{}
}

func (p *Publisher) Stop() error {
	if !p.isStarted() {
		return errors.New("publisher not started")
	}

	// Channel we'll use to determine that worker group is done
	doneCh := make(chan struct{})

	p.internalShutdownCancel()

	go func() {
		p.workerWg.Wait()
		close(p.workCh)
		doneCh <- struct{}{}
	}()

	// Wait for 5s for workers to shutdown
	select {
	case <-time.After(WorkerShutdownTimeout):
		// Timeout has occurred and doneCh not called yet - publishers haven't exited
		return fmt.Errorf("timed out ('%s') waiting for workers to shutdown", WorkerShutdownTimeout)
	case <-doneCh:
		// Workers have shutdown before timeout
		break
	}

	p.setStarted(false)

	return nil
}

func (p *Publisher) isStarted() bool {
	p.startedMtx.RLock()
	defer p.startedMtx.RUnlock()

	return p.started
}

func (p *Publisher) setStarted(started bool) {
	p.startedMtx.Lock()
	defer p.startedMtx.Unlock()

	p.started = started
}
