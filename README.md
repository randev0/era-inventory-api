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

