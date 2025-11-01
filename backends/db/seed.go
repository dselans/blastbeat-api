package db

import (
	"context"

	"github.com/superpowerdotcom/go-common-lib/clog"
	"go.uber.org/zap"
)

func (d *DB) Seed(ctx context.Context,
	log clog.ICustomLog) error {
	logger := log.With(zap.String("method", "Seed"))
	logger.Info("Seeding database")

	// Any sort of seeding service-specific seeding

	logger.Info("Database seeding completed (genres are seeded via migrations)")
	return nil
}
