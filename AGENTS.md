# Distributed Payment Orchestration Platform

## Project Structure

```
.
├── cmd/api/main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration loading from .env
│   ├── database/                # GORM database connection
│   ├── models/                  # Domain models (GORM)
│   ├── repositories/            # Data access layer
│   └── httpapi/                 # HTTP handlers (Gin)
├── migrations/                  # Versioned SQL migrations
├── go.mod
├── go.sum
└── README.md
```

## Development

### Build
```bash
go build ./...
```

### Test
```bash
go test ./...
```

### Run
```bash
# Requires PostgreSQL running (see README for setup)
go run ./cmd/api
```

## Conventions

- **Go 1.27+**
- **Gin** for HTTP routing
- **GORM** for PostgreSQL ORM
- **godotenv** for .env file loading
- **Dependency Injection** - Handlers receive dependencies through constructors
- **Clean Architecture** - Clear separation between layers

## Architecture Rules

- `cmd/` - Application entry points only, minimal logic
- `internal/` - All application code, not importable by other projects
- `internal/config/` - Configuration loading and validation
- `internal/database/` - Database connection management
- `internal/models/` - Domain models (GORM structs)
- `internal/repositories/` - Data access interfaces and implementations
- `internal/httpapi/` - HTTP handlers and routes

## Database

- PostgreSQL with GORM
- AutoMigrate runs on startup for models
- SQL migrations live in `migrations/` and use the `NNNNNN_description.up.sql` and `NNNNNN_description.down.sql` naming pattern
- Connection pool configured via GORM
- Environment variables: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`

## Commit Messages

Use concise, imperative mood messages:
- `Add Payment model and repository`
- `Update config to load database variables`
- `Fix connection pool configuration`
