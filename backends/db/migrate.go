package db

import (
	"context"
	"io/fs"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/go-common-lib/clog"
	"go.uber.org/zap"

	"github.com/dselans/blastbeat-api/migrations"
)

func (d *DB) Migrate(ctx context.Context,
	log clog.ICustomLog) error {
	logger := log.With(zap.String("method", "Migrate"))
	logger.Info("Running database migrations")

	if err := d.createMigrationsTable(ctx); err != nil {
		return errors.Wrap(err, "failed to create migrations table")
	}

	migrationFiles, err := d.getMigrationFiles()
	if err != nil {
		return errors.Wrap(err, "failed to get migration files")
	}

	applied, err := d.getAppliedMigrations(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get applied migrations")
	}

	for _, migration := range migrationFiles {
		if applied[migration.DirName] {
			logger.Debug("Migration already applied",
				zap.String("migration", migration.DirName),
				zap.String("file", migration.Name))
			continue
		}

		logger.Info("Applying migration",
			zap.String("migration", migration.DirName),
			zap.String("file", migration.Name))

		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to begin transaction")
		}

		content, err := migrations.FS.ReadFile(migration.FullPath)
		if err != nil {
			tx.Rollback()
			return errors.Wrapf(err,
				"failed to read migration file: %s",
				migration.FullPath)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return errors.Wrapf(err,
				"failed to execute migration: %s",
				migration.DirName)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (name, applied_at) "+
				"VALUES ($1, NOW())",
			migration.DirName); err != nil {
			tx.Rollback()
			return errors.Wrapf(err,
				"failed to record migration: %s", migration.DirName)
		}

		if err := tx.Commit(); err != nil {
			return errors.Wrapf(err,
				"failed to commit migration: %s", migration.DirName)
		}

		logger.Info("Migration applied successfully",
			zap.String("migration", migration.DirName),
			zap.String("file", migration.Name))
	}

	logger.Info("All migrations completed")
	return nil
}

func (d *DB) createMigrationsTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	_, err := d.db.ExecContext(ctx, query)
	return err
}

func (d *DB) getMigrationFiles() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, err
	}

	var migrationFiles []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			dirEntries, err := fs.ReadDir(migrations.FS, dirName)
			if err != nil {
				continue
			}
			for _, fileEntry := range dirEntries {
				if !fileEntry.IsDir() &&
					strings.HasSuffix(fileEntry.Name(), ".sql") {
					migrationFiles = append(migrationFiles,
						migrationFile{
							Name:    fileEntry.Name(),
							DirName: dirName,
							FullPath: dirName + "/" +
								fileEntry.Name(),
						})
				}
			}
		}
	}

	sort.Slice(migrationFiles, func(i, j int) bool {
		return migrationFiles[i].DirName <
			migrationFiles[j].DirName
	})

	return migrationFiles, nil
}

func (d *DB) getAppliedMigrations(ctx context.Context) (
	map[string]bool, error) {
	applied := make(map[string]bool)
	rows, err := d.db.QueryContext(ctx,
		"SELECT name FROM schema_migrations")
	if err != nil {
		return applied, nil
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}

	return applied, rows.Err()
}

type migrationFile struct {
	Name     string
	DirName  string
	FullPath string
}
