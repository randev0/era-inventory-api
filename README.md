# Era Inventory API

A simple **Go + Postgres REST API** for managing an inventory system.  
Built with [Go Chi](https://github.com/go-chi/chi) for routing, [pgx](https://github.com/jackc/pgx) for PostgreSQL, and fully containerized with Docker.

## Project Status
See **[PROGRESS.md](./PROGRESS.md)** for the live milestone checklist and next steps.

---

## 🚀 Features

- **JWT Authentication & Role-Based Access Control**
  - Secure token-based authentication
  - Role-based permissions (org_admin, project_admin, viewer)
  - Organization isolation
- Health checks (`/health`, `/dbping`)
- Full CRUD for inventory items:
  - `POST   /items` → create (requires org_admin or project_admin)
  - `GET    /items` → list with pagination & filters
  - `GET    /items/{id}` → fetch one
  - `PUT    /items/{id}` → update (requires org_admin or project_admin)
  - `DELETE /items/{id}` → remove (requires org_admin)
- Full CRUD for sites, vendors, and projects (requires org_admin for write operations)
- Filters: search by query, type, site
- Pagination (`page`, `limit` params)
- Unique `asset_tag` constraint
- JSON responses, ready for frontend integration
- Dockerized with `docker-compose`

---

## 🔐 Authentication

### JWT Configuration
Set these environment variables for JWT authentication:

```bash
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_ISS=era-inventory-api
JWT_AUD=era-inventory-api
JWT_EXPIRY=24h
```

### Generating Test Tokens
Use the included JWT generator tool:

```bash
# Build the tool
go build -o jwtgen cmd/tools/jwtgen.go

# Generate a token (default: user=1, org=1, roles=org_admin)
./jwtgen

# Custom token
./jwtgen -user 123 -org 456 -roles "org_admin,project_admin" -expiry 60
```

### Using Tokens
Include the JWT token in the Authorization header:

```bash
curl -H "Authorization: Bearer <your-jwt-token>" http://localhost:8080/items
```

### Role Requirements
- **Read operations** (GET): No specific role required, just valid JWT
- **Write operations** (POST/PUT): Requires `org_admin` or `project_admin` role
- **Delete operations** (DELETE): Requires `org_admin` role

---

## 📂 Project Structure

```
├── cmd/
│   ├── api/          # Main API server
│   ├── tools/        # JWT generator tool
│   └── testmigrate/  # Test database migration runner
├── internal/
│   ├── auth/         # JWT authentication & middleware
│   ├── config/       # Configuration management
│   ├── models/       # Data models
│   ├── testutil/     # Test utilities
│   └── ...           # Business logic
├── db/
│   ├── migrations/   # Database migrations
│   └── seeds/        # Test data seeds
├── .github/workflows/ # CI/CD workflows
└── docker-compose.yml
```

## 🚀 Quickstart

### Development Setup
```bash
# Start the development stack
make dev-up

# Set environment variables
cp env.example .env
# Edit .env with your database credentials

# Run the API
DATABASE_URL=postgres://postgres:postgres@localhost:5432/era?sslmode=disable go run ./cmd/api
```

### Testing
```bash
# Run unit tests only
make test

# Run integration tests (requires Docker)
make test-int

# Clean up test database
make test-int-down
```

### Local Test Database
To run tests locally with the same database configuration as CI:

```bash
# Start a local PostgreSQL instance with Docker
docker run --rm -e POSTGRES_USER=era -e POSTGRES_PASSWORD=era -e POSTGRES_DB=era_test -p 5432:5432 postgres:16-alpine

# Set the test database URL
export TEST_DATABASE_URL=postgres://era:era@localhost:5432/era_test?sslmode=disable

# Run migrations
go run ./cmd/testmigrate

# Apply seeds
psql "$TEST_DATABASE_URL" -f db/seeds/001_minimal.sql

# Run integration tests
INTEGRATION=1 go test ./... -v -tags=integration
```

## 📊 Excel Upload & Import

The API supports bulk importing assets from Excel files through a mapping-driven import system.

### Features
- **Mapping-driven Import**: Uses YAML configuration to map Excel columns to database fields
- **Dry Run Support**: Test imports without making changes using `dry_run=true`
- **Idempotent Upserts**: Updates existing assets or creates new ones based on natural keys
- **Error Handling**: Detailed error reporting with row-level feedback
- **Organization Isolation**: Imports are scoped to the authenticated user's organization
- **Role-based Access**: Requires `project_admin` or `org_admin` role

### Upload Excel File

```bash
# Get authentication token
make login EMAIL=admin@example.com PASSWORD=password

# Test with dry run (no changes made)
make test-upload-dry-run TK=your-token SITE=5 FILE=./testdata/sample.xlsx

# Real import (makes actual changes)
make test-upload-real TK=your-token SITE=5 FILE=./testdata/sample.xlsx
```

### Manual curl Examples

```bash
# Dry run import
curl -X POST http://localhost:8080/api/v1/imports/excel \
  -H "Authorization: Bearer $TOKEN" \
  -F dry_run=true \
  -F site_id=5 \
  -F file=@./data/Master\ List\ -\ MBIP\ MEDINI.xlsx | jq

# Real import
curl -X POST http://localhost:8080/api/v1/imports/excel \
  -H "Authorization: Bearer $TOKEN" \
  -F site_id=5 \
  -F file=@./data/Master\ List\ -\ MBIP\ MEDINI.xlsx | jq

# With custom mapping and error limit
curl -X POST http://localhost:8080/api/v1/imports/excel \
  -H "Authorization: Bearer $TOKEN" \
  -F site_id=5 \
  -F mapping=configs/mapping/custom.yaml \
  -F max_errors=100 \
  -F file=@./data/assets.xlsx | jq
```

### File Requirements
- **Format**: Only `.xlsx` files are accepted
- **Size Limit**: Maximum 20 MB file size
- **Content Type**: Must be `multipart/form-data`

### Import Process
1. **Validation**: File format and size validation
2. **Mapping**: Excel headers mapped to database fields using YAML configuration
3. **Parsing**: Data types parsed (IP, CIDR, INT, BOOL, TIMESTAMP, TEXT)
4. **Upsert**: Assets created or updated based on natural keys (serial, name, etc.)
5. **Subtypes**: Switch and VLAN data stored in subtype tables when applicable
6. **Extras**: Unknown columns stored in JSONB `extras` field

### CLI Tool Alternative

You can also use the command-line importer tool:

```bash
# Build the tool
make build-import-excel

# Import with dry run
./bin/import-excel --file=./data/assets.xlsx --org-id=1 --site-id=5 --dry-run

# Real import
./bin/import-excel --file=./data/assets.xlsx --org-id=1 --site-id=5
```

## 🔧 Development Tools

### Makefile Targets
```bash
make help          # Show all available targets
make dev-up        # Start development stack
make dev-down      # Stop development stack
make test          # Run unit tests
make test-int      # Run integration tests
make test-int-up   # Start test database
make test-int-down # Stop test database
make openapi       # Generate OpenAPI docs
make build         # Build binary
make clean         # Clean build artifacts
```

## 📊 Observability

### Metrics Endpoint
- **Endpoint**: `GET /metrics`
- **Format**: Prometheus metrics
- **Control**: Set `ENABLE_METRICS=true` to enable

### OpenAPI Documentation
- **Spec**: `GET /openapi.yaml`
- **UI**: `GET /docs` (Swagger UI)
- **Control**: Set `ENABLE_SWAGGER=true` to enable

## 🚀 CI/CD

The project includes GitHub Actions workflows that:
- Run unit tests on every push/PR
- Run integration tests on main branch
- Include security scanning with Trivy
- Upload test coverage reports

**Local testing**: Use `make test-int` to run the same integration tests locally.

## 🧩 Migrations

- Run migrations: `docker compose up migrate`
- Re-run a specific migration manually:
  - `docker exec -it <db_container> psql -U postgres -d era -f /migrations/0001_inventory.sql`
- Verify tables exist:
  - `docker exec -it <db_container> psql -U postgres -d era -c "\dt"`

