package config

import (
	"fmt"
	"reflect"
	"time"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	EnvFile         = ".env"
	EnvConfigPrefix = "BLASTBEAT_API"
)

type Config struct {
	Version          kong.VersionFlag `help:"Show version and exit" short:"v" env:"-"`
	EnvName          string           `kong:"help='Environment name.',default='dev'"`
	ServiceName      string           `kong:"help='Service name.',default='blastbeat-api'"`
	HealthFreqSec    int              `kong:"help='Health check frequency in seconds.',default=10"`
	EnablePprof      bool             `kong:"help='Enable pprof endpoints (http://$apiListenAddress/debug).',default=false"`
	APIListenAddress string           `kong:"help='API listen address (serves health, metrics, version).',default=:8080"`
	LogConfig        string           `kong:"help='Logging config to use.',enum='dev,prod',default='dev'"`

	NewRelicAppName    string `kong:"help='New Relic application name.',default='blastbeat-api (DEV)'"`
	NewRelicLicenseKey string `kong:"help='New Relic license key.'"`

	DBHost     string `kong:"help='Database host.',default=localhost"`
	DBName     string `kong:"help='Database name.',default=blastbeat"`
	DBUser     string `kong:"help='Database user.',default=blastbeat"`
	DBPassword string `kong:"help='Database password.',default=blastbeat"`
	DBPort     int    `kong:"help='Database port.',default=5432"`
	DBSSLMode  string `kong:"help='Database SSL mode.',default=disable"`

	RedisURL         string        `kong:"help='Redis URL.',default=localhost:6379"`
	RedisPassword    string        `kong:"help='Redis Password.'"`
	RedisDatabase    int           `kong:"help='Redis database.',default=0"`
	RedisPoolSize    int           `kong:"help='Redis pool size.',default=10"`
	RedisDialTimeout time.Duration `kong:"help='Redis dial timeout.',default=5s"`

	KongContext *kong.Context `kong:"-"`
}

func New(version string) *Config {
	if err := godotenv.Load(EnvFile); err != nil {
		zap.L().Warn("unable to load dotenv file",
			zap.String("err", err.Error()))
	}

	cfg := &Config{}
	cfg.KongContext = kong.Parse(
		cfg,
		kong.Name("blastbeat-api"),
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

func (c *Config) GetMap() map[string]string {
	fields := make(map[string]string)

	val := reflect.ValueOf(c)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	t := val.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := val.Field(i)
		fields[field.Name] = fmt.Sprintf("%v", value)
	}

	return fields
}
