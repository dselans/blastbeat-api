package deps

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"github.com/InVisionApp/go-health"
	"github.com/bsm/redislock"
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrzap"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/streamdal/rabbit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	ss "github.com/your_org/go-svc-template/services/state"

	sb "github.com/your_org/go-svc-template/backends/state"

	"github.com/your_org/go-svc-template/backends/cache"
	"github.com/your_org/go-svc-template/clog"
	"github.com/your_org/go-svc-template/config"
	"github.com/your_org/go-svc-template/services/processor"
	"github.com/your_org/go-svc-template/services/publisher"
)

const (
	DefaultHealthCheckIntervalSecs = 1
)

type customCheck struct{}

type Dependencies struct {
	// Backends
	ProcessorRabbitBackend rabbit.IRabbit
	PublisherRabbitBackend rabbit.IRabbit
	CacheBackend           cache.ICache
	RedisBackend           *redis.Client
	RedisLockBackend       *redislock.Client
	StateBackend           sb.IState

	// Services
	ProcessorService processor.IProcessor
	PublisherService publisher.IPublisher
	StateService     ss.IState

	Health health.IHealth

	// Global, shared shutdown context - all services and backends listen to
	// this context to know when to shutdown.
	ShutdownCtx context.Context

	// ShutdownCancel is the cancel function for the global shutdown context
	ShutdownCancel context.CancelFunc

	// Channel written to by publisher when it's done shutting down; read by
	// shutdown handler in main(). We need this so that we can tell the shutdown
	// handler when it is safe to exit.
	PublisherShutdownDoneCh chan struct{}

	NewRelicApp *newrelic.Application
	Config      *config.Config

	// Log is the main, shared logger (you should use this for all logging)
	Log clog.ICustomLog

	// ZapLog is the zap logger (you shouldn't need this outside of deps)
	ZapLog *zap.Logger

	// ZapCore can be used to generate a brand-new logger (you shouldn't need this very often)
	ZapCore zapcore.Core
}

func New(cfg *config.Config) (*Dependencies, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Dependencies{
		ShutdownCtx:             ctx,
		ShutdownCancel:          cancel,
		PublisherShutdownDoneCh: make(chan struct{}),
		Config:                  cfg,
	}

	// NewRelic setup must occur before logging setup
	if err := d.setupNewRelic(); err != nil {
		return nil, errors.Wrap(err, "unable to setup newrelic")
	}

	if err := d.setupLogging(); err != nil {
		return nil, errors.Wrap(err, "unable to setup logging")
	}

	if err := d.setupHealth(); err != nil {
		return nil, errors.Wrap(err, "unable to setup health")
	}

	if err := d.Health.Start(); err != nil {
		return nil, errors.Wrap(err, "unable to start health runner")
	}

	if err := d.setupBackends(cfg); err != nil {
		return nil, errors.Wrap(err, "unable to setup backends")
	}

	if err := d.setupServices(cfg); err != nil {
		return nil, errors.Wrap(err, "unable to setup services")
	}

	return d, nil
}

func (d *Dependencies) setupNewRelic() error {
	if d.Config.NewRelicAppName == "" || d.Config.NewRelicLicenseKey == "" {
		return nil
	}
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(d.Config.NewRelicAppName),
		newrelic.ConfigLicense(d.Config.NewRelicLicenseKey),
		newrelic.ConfigAppLogForwardingEnabled(true),
		newrelic.ConfigZapAttributesEncoder(true),
	)

	if err != nil {
		return errors.Wrap(err, "unable to create newrelic app")
	}

	if err := app.WaitForConnection(10 * time.Second); err != nil {
		return errors.Wrap(err, "unable to connect to newrelic")
	}

	d.NewRelicApp = app

	return nil
}

// If using New Relic, setupLogging() should be called _after_ setupNewRelic()
func (d *Dependencies) setupLogging() error {
	var core zapcore.Core

	if d.Config.LogConfig == "dev" {
		zc := zap.NewDevelopmentConfig()
		zc.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		core = zapcore.NewCore(zapcore.NewConsoleEncoder(zc.EncoderConfig),
			zapcore.AddSync(os.Stdout),
			zap.DebugLevel,
		)
	} else {
		core = zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(os.Stdout),
			zap.InfoLevel,
		)
	}

	// If using New Relic, wrap zap core with New Relic core
	if d.NewRelicApp != nil {
		var err error

		core, err = nrzap.WrapBackgroundCore(core, d.NewRelicApp)
		if err != nil {
			return errors.Wrap(err, "unable to wrap zap core with newrelic")
		}
	}

	// Save the actual loggers
	d.ZapLog = zap.New(core)
	d.ZapCore = core

	// Create a new primary logger that will be passed to everyone
	d.Log = clog.New(d.ZapLog, zap.String("env", d.Config.EnvName))

	d.Log.Debug("Logging initialized")

	return nil
}

func (d *Dependencies) setupHealth() error {
	logger := d.Log.With(zap.String("method", "setupHealth"))
	logger.Debug("Setting up health")

	gohealth := health.New()
	gohealth.DisableLogging()

	cc := &customCheck{}

	err := gohealth.AddChecks([]*health.Config{
		{
			Name:     "health-check",
			Checker:  cc,
			Interval: time.Duration(DefaultHealthCheckIntervalSecs) * time.Second,
			Fatal:    true,
		},
	})

	d.Health = gohealth

	if err != nil {
		return err
	}

	return nil
}

func (d *Dependencies) setupBackends(cfg *config.Config) error {
	llog := d.Log.With(zap.String("method", "setupBackends"))

	llog.Debug("Setting up cache backend")

	// CacheBackend k/v store
	cb, err := cache.New()
	if err != nil {
		return errors.Wrap(err, "unable to create new cache instance")
	}

	d.CacheBackend = cb

	llog.Debug("Setting up redis backend")

	// Redis backend for state
	client := redis.NewClient(&redis.Options{
		Addr:        cfg.RedisURL,
		Password:    cfg.RedisPassword,
		DB:          cfg.RedisDatabase,
		PoolSize:    cfg.RedisPoolSize,
		DialTimeout: cfg.RedisDialTimeout,
	})

	if err := client.ClientInfo(d.ShutdownCtx).Err(); err != nil {
		return errors.Wrap(err, "unable to connect to redis")
	}

	d.RedisBackend = client

	// Create redislock backend
	d.RedisLockBackend = redislock.New(d.RedisBackend)

	// Create state backend
	s, err := sb.New(&sb.Options{
		Prefix:      cfg.ServiceName,
		Log:         d.Log,
		RedisClient: d.RedisBackend,
		RedisLock:   d.RedisLockBackend,
	})
	if err != nil {
		return errors.Wrap(err, "unable to setup state service")
	}

	d.StateBackend = s

	llog.Debug("Setting up rabbit backend")

	// RabbitMQ backend for processing messages
	procRabbitBackend, err := rabbit.New(&rabbit.Options{
		URLs:      cfg.ProcessorRabbitURL,
		Mode:      1,
		QueueName: cfg.ProcessorRabbitQueueName,
		Bindings: []rabbit.Binding{
			{
				ExchangeName:    cfg.ProcessorRabbitExchangeName,
				ExchangeType:    cfg.ProcessorRabbitExchangeType,
				ExchangeDeclare: cfg.ProcessorRabbitExchangeDeclare,
				ExchangeDurable: cfg.ProcessorRabbitExchangeDurable,
				BindingKeys:     cfg.ProcessorRabbitBindingKeys,
			},
		},
		RetryReconnectSec: rabbit.DefaultRetryReconnectSec,
		QueueDurable:      cfg.ProcessorRabbitQueueDurable,
		QueueExclusive:    cfg.ProcessorRabbitQueueExclusive,
		QueueAutoDelete:   cfg.ProcessorRabbitQueueAutoDelete,
		QueueDeclare:      cfg.ProcessorRabbitQueueDeclare,
		AutoAck:           cfg.ProcessorRabbitAutoAck,
		AppID:             cfg.ServiceName + "-processor",
		UseTLS:            cfg.ProcessorRabbitUseTLS,
		SkipVerifyTLS:     cfg.ProcessorRabbitSkipVerifyTLS,
		Log: d.ZapLog.Sugar().With(
			zap.String("env", cfg.EnvName),
			zap.String("pkg", "rabbit"),
			zap.String("backend", "processor"),
		),
	})
	if err != nil {
		return errors.Wrap(err, "unable to create rabbit backend for processor")
	}

	d.ProcessorRabbitBackend = procRabbitBackend

	// RabbitMQ backend for publishing
	pubRabbitBackend, err := rabbit.New(&rabbit.Options{
		URLs: cfg.PublisherRabbitURL,
		Bindings: []rabbit.Binding{
			{
				ExchangeName:       cfg.PublisherRabbitExchangeName,
				ExchangeType:       cfg.PublisherRabbitExchangeType,
				ExchangeDeclare:    cfg.PublisherRabbitExchangeDeclare,
				ExchangeDurable:    cfg.PublisherRabbitExchangeDurable,
				ExchangeAutoDelete: cfg.PublisherRabbitExchangeAutoDelete,
			},
		},
		Mode:              rabbit.Producer,
		RetryReconnectSec: rabbit.DefaultRetryReconnectSec,
		AppID:             cfg.ServiceName + "-publisher",
		UseTLS:            cfg.PublisherRabbitUseTLS,
		SkipVerifyTLS:     cfg.PublisherRabbitSkipVerifyTLS,
		Log: d.ZapLog.Sugar().With(
			zap.String("env", cfg.EnvName),
			zap.String("pkg", "rabbit"),
			zap.String("backend", "rabbit-publisher"),
		),
	})
	if err != nil {
		return errors.Wrap(err, "unable to create rabbit backend for publisher")
	}

	d.PublisherRabbitBackend = pubRabbitBackend

	return nil
}

func (d *Dependencies) setupServices(cfg *config.Config) error {
	logger := d.Log.With(zap.String("method", "setupServices"))
	logger.Debug("Setting up services")

	logger.Debug("Setting up state service")

	// Setup state service
	s, err := ss.New(&ss.Options{
		Backend:     d.StateBackend,
		Log:         d.Log,
		ShutdownCtx: d.ShutdownCtx,
		Cache:       d.CacheBackend,
	})
	if err != nil {
		return errors.Wrap(err, "unable to setup state service")
	}

	d.StateService = s

	logger.Debug("Setting up processor service")

	// Setup service that will consume and process messages from RabbitMQ
	procService, err := processor.New(&processor.Options{
		Cache: d.CacheBackend,
		RabbitMap: map[string]*processor.RabbitConfig{
			"main": {
				RabbitInstance: d.ProcessorRabbitBackend,
				NumConsumers:   cfg.ProcessorRabbitNumConsumers,
				Func:           "ConsumeFunc",
			},
		},
		// TODO: Instrument graceful shutdown
		NewRelic:     d.NewRelicApp,
		Log:          d.Log,
		ShutdownCtx:  d.ShutdownCtx,
		StateService: d.StateService,
	}, cfg)
	if err != nil {
		return errors.Wrap(err, "unable to setup proc service")
	}

	d.ProcessorService = procService

	// Setup service that will publish messages to RabbitMQ
	pubService, err := publisher.New(&publisher.Options{
		RabbitBackend:          d.PublisherRabbitBackend,
		NumWorkers:             cfg.PublisherNumWorkers,
		ExternalShutdownCtx:    d.ShutdownCtx,
		ExternalShutdownDoneCh: d.PublisherShutdownDoneCh,
		NewRelic:               d.NewRelicApp,
		Log:                    d.Log,
	})
	if err != nil {
		return errors.Wrap(err, "unable to create new publisher")
	}

	if err := pubService.Start(); err != nil {
		return errors.Wrap(err, "unable to start publisher")
	}

	d.PublisherService = pubService

	return nil
}

func createTLSConfig(caCert, clientCert, clientKey string) (*tls.Config, error) {
	cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
	if err != nil {
		return nil, errors.Wrap(err, "unable to load cert + key")
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}, nil
}

// Status satisfies the go-health.ICheckable interface
func (c *customCheck) Status() (interface{}, error) {
	if false {
		return nil, errors.New("something major just broke")
	}

	// You can return additional information pertaining to the check as long
	// as it can be JSON marshalled
	return map[string]int{}, nil
}
