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

	ProcessorRabbitURL               []string `kong:"help='RabbitMQ server URL(s).',default=amqp://localhost"`
	ProcessorRabbitExchangeName      string   `kong:"help='RabbitMQ exchange name',default=events"`
	ProcessorRabbitExchangeDeclare   bool     `kong:"help='Whether to declare/create exchange if it does not already exist.',default=true"`
	ProcessorRabbitExchangeDurable   bool     `kong:"help='Whether exchange should survive a RabbitMQ server restart.',default=true"`
	ProcessorRabbitExchangeType      string   `kong:"help='RabbitMQ exchange type.',enum='direct,fanout,topic,headers',default=topic"`
	ProcessorRabbitBindingKeys       []string `kong:"help='Bind the following routing-keys to the queue-name.',default='data-proc'"`
	ProcessorRabbitQueueName         string   `kong:"help='RabbitMQ queue name.',default='data-proc'"`
	ProcessorRabbitNumConsumers      int      `kong:"help='Number of RabbitMQ consumers.',default=4"`
	ProcessorRabbitRetryReconnectSec int      `kong:"help='Interval used for re-connecting to Rabbit (when it goes away).',default=10"`
	ProcessorRabbitAutoAck           bool     `kong:"help='Whether to auto-ACK consumed messages. You probably do not want this.',default=false"`
	ProcessorRabbitQueueDeclare      bool     `kong:"help='Whether to declare/create queue if it does not already exist.',default=true"`
	ProcessorRabbitQueueDurable      bool     `kong:"help='Whether queue and its contents should survive a RabbitMQ server restart.',default=true"`
	ProcessorRabbitQueueExclusive    bool     `kong:"help='Whether the queue should only allow 1 specific consumer. You probably do not want this.',default=false"`
	ProcessorRabbitQueueAutoDelete   bool     `kong:"help='Whether to auto-delete queue when there are no attached consumers. You probably do not want this.',default=false"`
	ProcessorRabbitUseTLS            bool     `kong:"help='RabbitMQ use TLS.',default=false"`
	ProcessorRabbitSkipVerifyTLS     bool     `kong:"help='RabbitMQ skip TLS verification.',default=false"`

	PublisherRabbitURL                []string `kong:"help='RabbitMQ server URL(s).',default=amqp://localhost"`
	PublisherRabbitExchangeName       string   `kong:"help='RabbitMQ exchange name',default=events"`
	PublisherRabbitExchangeDeclare    bool     `kong:"help='Whether to declare/create exchange if it does not already exist.',default=true"`
	PublisherRabbitExchangeDurable    bool     `kong:"help='Whether exchange should survive a RabbitMQ server restart.',default=true"`
	PublisherRabbitExchangeAutoDelete bool     `kong:"help='Whether to auto-delete exchange when there are no attached queues. You probably do not want this.',default=false"`
	PublisherRabbitExchangeType       string   `kong:"help='RabbitMQ exchange type.',enum='direct,fanout,topic,headers',default=topic"`
	PublisherRabbitRetryReconnectSec  int      `kong:"help='Interval used for re-connecting to Rabbit (when it goes away).',default=10"`
	PublisherRabbitUseTLS             bool     `kong:"help='RabbitMQ use TLS.',default=false"`
	PublisherRabbitSkipVerifyTLS      bool     `kong:"help='RabbitMQ skip TLS verification.',default=false"`
	PublisherNumWorkers               int      `kong:"help='Number of publisher workers to run.',default=10"`

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
