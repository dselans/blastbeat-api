package deps

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/InVisionApp/go-health"
	"github.com/bsm/redislock"
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrzap"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/superpowerdotcom/go-common-lib/clog"

	"github.com/dselans/blastbeat-api/backends/db"
	sb "github.com/dselans/blastbeat-api/backends/state"
	"github.com/dselans/blastbeat-api/config"
	ss "github.com/dselans/blastbeat-api/services/state"
)

const (
	DefaultHealthCheckIntervalSecs = 1
)

type customCheck struct{}

type Dependencies struct {
	// Backends
	RedisBackend     *redis.Client
	RedisLockBackend *redislock.Client
	StateBackend     sb.IState
	DBBackend        *db.DB

	// Services
	StateService ss.IState

	Health health.IHealth

	// Global, shared shutdown context - all services and backends listen to
	// this context to know when to shutdown.
	ShutdownCtx context.Context

	// ShutdownCancel is the cancel function for the global shutdown context
	ShutdownCancel context.CancelFunc

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
		ShutdownCtx:    ctx,
		ShutdownCancel: cancel,
		Config:         cfg,
	}

	// NewRelic setup must occur before logging setup
	if err := d.setupNewRelic(); err != nil {
		return nil, errors.Wrap(err, "unable to setup newrelic")
	}

	if err := d.setupLogging(); err != nil {
		return nil, errors.Wrap(err, "unable to setup logging")
	}

	// Pretty print config in dev mode
	if d.Config.LogConfig == "dev" {
		d.LogConfig()
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

	// Setup database backend
	llog.Debug("Setting up database backend")

	db2, err := db.New(&db.Options{
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		DBName:   cfg.DBName,
	})
	if err != nil {
		return errors.Wrap(err, "unable to setup database backend")
	}

	d.DBBackend = db2

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
	})
	if err != nil {
		return errors.Wrap(err, "unable to setup state service")
	}

	d.StateService = s

	logger.Debug("Setting up processor service")

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

// LogConfig pretty prints the config to the log
func (d *Dependencies) LogConfig() {
	d.ZapLog.Info("Config")

	longestKey := 0

	for k, _ := range d.Config.GetMap() {
		if len(k) > longestKey {
			longestKey = len(k)
		}
	}

	maxPadding := longestKey + 3
	totalKeys := len(d.Config.GetMap())
	index := 0
	prefix := "├─"

	for k, v := range d.Config.GetMap() {
		index++

		if index == totalKeys {
			prefix = "└─"
		}

		padding := maxPadding - len(k)

		line := fmt.Sprintf("%s %s %s %-"+strconv.Itoa(len(k))+"v", prefix, k, strings.Repeat(" ", padding), v)
		d.ZapLog.Debug(line)
	}
}
