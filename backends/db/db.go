package db

import (
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"

	"github.com/dselans/blastbeat-api/backends/gensql"
)

type Options struct {
	User     string
	Password string
	Host     string
	Port     int
	DBName   string
}

type DB struct {
	// Only becomes available after New() returns successfully.
	*gensql.Queries

	opts *Options
	db   *sql.DB
}

const DefaultPostgreSQLPort = 5432

func New(opts *Options) (*DB, error) {
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, "invalid options")
	}

	// Try to connect to db
	dsn := fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s sslmode=verify-full")

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse database connection string")
	}

	db := stdlib.OpenDB(*cfg.ConnConfig)
	queries := gensql.New(db)

	return &DB{
		Queries: queries,
		db:      db,
	}, nil
}

func validateOptions(opts *Options) error {
	if opts == nil {
		return errors.New("options cannot be nil")
	}

	if opts.User == "" {
		return errors.New("user cannot be empty")
	}

	if opts.Password == "" {
		return errors.New("password cannot be empty")
	}

	if opts.Host == "" {
		return errors.New("host cannot be empty")
	}

	if opts.Port <= 0 {
		opts.Port = DefaultPostgreSQLPort
	}

	return nil
}
