# Distributed Payment Orchestration Platform

A distributed platform for orchestrating payment workflows.

## Development

### Prerequisites

- Go 1.21+
- Podman or Docker
- PostgreSQL

### Quick Start

1. **Clone the repository:**
   ```bash
   git clone git@github.com:naet001/distributed-payment-orchestration-platform.git
   cd distributed-payment-orchestration-platform
   ```

2. **Start PostgreSQL:**
   ```bash
   podman run -d \
     --name payment-db \
     -e POSTGRES_USER=payment_user \
     -e POSTGRES_PASSWORD=payment_pass \
     -e POSTGRES_DB=payment_db \
     -p 5432:5432 \
     docker.io/library/postgres:latest
   ```

3. **Configure environment:**
   ```bash
   cp .env.example .env
   ```
   Edit `.env` with your database credentials.

4. **Install dependencies:**
   ```bash
   go mod download
   ```

5. **Run the server:**
   ```bash
   go run ./cmd/api
   ```

6. **Test the health endpoint:**
   ```bash
   curl http://localhost:8080/health
   ```

### Project Structure

```
.
├── cmd/
│   └── api/                    # Application entry point
│       └── main.go
├── internal/
│   ├── config/                 # Configuration loading
│   │   └── config.go
│   ├── database/               # Database connection
│   │   └── connection.go
│   └── httpapi/                # HTTP handlers
│       ├── handler.go
│       └── handler_test.go
├── .env.example                # Example environment variables
├── go.mod                      # Go module
├── go.sum                      # Dependency checksums
└── README.md
```

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | HTTP server port |
| DB_HOST | localhost | Database host |
| DB_PORT | 5432 | Database port |
| DB_USER | - | Database username |
| DB_PASSWORD | - | Database password |
| DB_NAME | - | Database name |
| DB_SSL_MODE | disable | SSL mode for database connection |

### Running Tests

```bash
# Run all tests (unit tests will run, integration tests require TEST_DATABASE_DSN)
go test ./...

# Run integration tests
TEST_DATABASE_DSN="postgres://payment_user:payment_pass@localhost:5432/payment_db?sslmode=disable" go test ./internal/repositories/... -v
```

**Note:** Integration tests in `internal/repositories/payment_integration_test.go` require a PostgreSQL database connection string in the `TEST_DATABASE_DSN` environment variable. These tests are skipped if the variable is not set.

## Architecture

- **Dependency Injection:** Handlers receive dependencies through constructors
- **Clean Architecture:** Clear separation between layers
- **GORM:** ORM for PostgreSQL database interactions
- **Gin:** HTTP web framework

## License

MIT
