# blastbeat-api

Go backend API powering [blastbeat.io](https://blastbeat.io).

## Structure

The codebase follows a layered architecture:

- **`api/`** - HTTP handlers that receive requests and return responses
- **`services/`** - Business logic layer (e.g., `services/release/` filters releases)
- **`backends/db/`** - Database connection and migrations
- **`backends/gensql/`** - Generated SQL code from sqlc queries
- **`deps/`** - Dependency injection that wires everything together

Flow: HTTP Request → Handler → Service → Database Backend → PostgreSQL

## Database Migrations

Migrations run automatically when the service starts. Each migration is
a numbered directory under `migrations/` containing a SQL file and a
README.md explaining what the migration does.

### Migration Structure

Each migration is in its own directory:

```
migrations/
├── 001_initial_schema/
│   ├── 001_initial_schema.sql
│   └── README.md
└── 002_seed_genres/
    ├── 002_seed_genres.sql
    └── README.md
```

### How Migrations Work

The migration system is embedded in the Go binary and runs
automatically on service startup:

1. **Startup Process**: When `deps.New()` initializes the database
   backend, it automatically calls `db.Migrate()`.

2. **Tracking Table**: The system first creates a `schema_migrations`
   table (if it doesn't exist) to track which migrations have been
   applied. This table stores:

   - `name` - The migration directory name (e.g., `001_initial_schema`)
   - `applied_at` - Timestamp when the migration was run

3. **Migration Discovery**: All migration directories in `migrations/`
   are discovered and sorted alphabetically by directory name. This
   ensures numeric ordering (001, 002, 003, etc.).

4. **Execution Logic**: For each migration directory:

   - Check if it's already in `schema_migrations` table
   - If already applied, skip it
   - If not applied, run it:
     - Begin a database transaction
     - Execute all SQL in the migration's `.sql` file
     - Record the migration directory name in `schema_migrations`
     - Commit the transaction
   - If any step fails, the transaction rolls back and the service
     fails to start

5. **Idempotency**: Migrations are safe to run multiple times.
   Already-applied migrations are automatically skipped based on the
   tracking table.

6. **Transaction Safety**: Each migration runs in its own transaction,
   so either the entire migration succeeds or nothing is applied
   (atomic operation).

### Creating New Migrations

Use the Makefile helper:

```bash
make migration/new NAME=add_user_preferences
```

This creates a new numbered migration directory in `migrations/` with:

- A SQL file template
- A README.md template explaining what the migration does
