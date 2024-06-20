package deps

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"github.com/InVisionApp/go-health"
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrzap"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/streamdal/rabbit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/your_org/go-svc-template/backends/cache"
	"github.com/your_org/go-svc-template/clog"
	"github.com/your_org/go-svc-template/config"
	"github.com/your_org/go-svc-template/services/proc"
)

const (
	DefaultHealthCheckIntervalSecs = 1
)

type customCheck struct{}

type Dependencies struct {
	// Backends
	RabbitBackend rabbit.IRabbit
	CacheBackend  cache.ICache

	// Services
	ProcessorService proc.IProc

	Health         health.IHealth
	DefaultContext context.Context

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
	d := &Dependencies{
		DefaultContext: context.Background(),
		Config:         cfg,
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

	llog.Debug("Setting up rabbit backend")

	// Rabbitmq backend
	rabbitBackend, err := rabbit.New(&rabbit.Options{
		URLs:      cfg.RabbitURL,
		Mode:      1,
		QueueName: cfg.RabbitQueueName,
		Bindings: []rabbit.Binding{
			{
				ExchangeName:    cfg.RabbitExchangeName,
				ExchangeType:    amqp.ExchangeTopic,
				ExchangeDeclare: cfg.RabbitExchangeDeclare,
				BindingKeys:     cfg.RabbitBindingKeys,
			},
		},
		RetryReconnectSec: rabbit.DefaultRetryReconnectSec,
		QueueDurable:      cfg.RabbitQueueDurable,
		QueueExclusive:    cfg.RabbitQueueExclusive,
		QueueAutoDelete:   cfg.RabbitQueueAutoDelete,
		QueueDeclare:      cfg.RabbitQueueDeclare,
		AutoAck:           cfg.RabbitAutoAck,
		AppID:             cfg.ServiceName,
		UseTLS:            cfg.RabbitUseTLS,
		SkipVerifyTLS:     cfg.RabbitSkipVerifyTLS,
	})
	if err != nil {
		return errors.Wrap(err, "unable to create new dedicated rabbit backend")
	}

	d.RabbitBackend = rabbitBackend

	return nil
}

func (d *Dependencies) setupServices(cfg *config.Config) error {
	logger := d.Log.With(zap.String("method", "setupServices"))
	logger.Debug("Setting up services")

	procService, err := proc.New(&proc.Options{
		Cache: d.CacheBackend,
		RabbitMap: map[string]*proc.RabbitConfig{
			"main": {
				RabbitInstance: d.RabbitBackend,
				NumConsumers:   cfg.RabbitNumConsumers,
				Func:           "MainConsumeFunc",
			},
		},
		NewRelic: d.NewRelicApp,
		Log:      d.Log,
	}, cfg)
	if err != nil {
		return errors.Wrap(err, "unable to setup proc service")
	}

	d.ProcessorService = procService

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
