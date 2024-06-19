package config

import (
	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	EnvFile         = ".env"
	EnvConfigPrefix = "GO_SVC_TEMPLATE"
)

type Config struct {
	Version          kong.VersionFlag `help:"Show version and exit" short:"v" env:"-"`
	EnvName          string           `kong:"help='Environment name.',default='dev'"`
	ServiceName      string           `kong:"help='Service name.',default='go-svc-template'"`
	HealthFreqSec    int              `kong:"help='Health check frequency in seconds.',default=10"`
	EnablePprof      bool             `kong:"help='Enable pprof endpoints (http://$apiListenAddress/debug).',default=false"`
	APIListenAddress string           `kong:"help='API listen address (serves health, metrics, version).',default=:8080"`
	LogConfig        string           `kong:"help='Logging config to use.',enum='dev,prod',default='dev'"`

	NewRelicAppName    string `kong:"help='New Relic application name.',default='go-svc-template (DEV)'"`
	NewRelicLicenseKey string `kong:"help='New Relic license key.'"`

	RabbitURL               []string `kong:"help='RabbitMQ server URL(s).',default=amqp://localhost"`
	RabbitExchangeName      string   `kong:"help='RabbitMQ exchange name',default=events"`
	RabbitExchangeDeclare   bool     `kong:"help='Whether to declare/create exchange if it does not already exist.',default=true"`
	RabbitExchangeDurable   bool     `kong:"help='Whether exchange should survive a RabbitMQ server restart.',default=true"`
	RabbitBindingKeys       []string `kong:"help='Bind the following routing-keys to the queue-name.',default='data-proc'"`
	RabbitQueueName         string   `kong:"help='RabbitMQ queue name.',default='data-proc'"`
	RabbitNumConsumers      int      `kong:"help='Number of RabbitMQ consumers.',default=4"`
	RabbitRetryReconnectSec int      `kong:"help='Interval used for re-connecting to Rabbit (when it goes away).',default=10"`
	RabbitAutoAck           bool     `kong:"help='Whether to auto-ACK consumed messages. You probably do not want this.',default=false"`
	RabbitQueueDeclare      bool     `kong:"help='Whether to declare/create queue if it does not already exist.',default=true"`
	RabbitQueueDurable      bool     `kong:"help='Whether queue and its contents should survive a RabbitMQ server restart.',default=true"`
	RabbitQueueExclusive    bool     `kong:"help='Whether the queue should only allow 1 specific consumer. You probably do not want this.',default=false"`
	RabbitQueueAutoDelete   bool     `kong:"help='Whether to auto-delete queue when there are no attached consumers. You probably do not want this.',default=false"`
	RabbitUseTLS            bool     `kong:"help='RabbitMQ use TLS.',default=false,short='t'"`
	RabbitSkipVerifyTLS     bool     `kong:"help='RabbitMQ skip TLS verification.',default=false"`

	KongContext *kong.Context `kong:"-"`
}

func New(version string) *Config {
	// Attempt to load .env - do not fail if it's not there. Only environment
	// that might have this is in local/dev; staging, prod should not have one.
	if err := godotenv.Load(EnvFile); err != nil {
		zap.L().Warn("unable to load dotenv file", zap.String("err", err.Error()))
	}

	cfg := &Config{}
	cfg.KongContext = kong.Parse(
		cfg,
		kong.Name("go-svc-template"),
		kong.Description("Golang service"),
		kong.DefaultEnvars(EnvConfigPrefix),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}),
		kong.Vars{
			"version": version,
		},
	)

	return cfg
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("Config cannot be nil")
	}

	return nil
}
