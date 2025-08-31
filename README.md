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
│   └── tools/        # JWT generator tool
├── internal/
│   ├── auth/         # JWT authentication & middleware
│   ├── config/       # Configuration management
│   ├── models/       # Data models
│   └── ...           # Business logic
├── db/               # Database migrations
└── docker-compose.yml
```

## 🧩 Migrations

- Run migrations: `docker compose up migrate`
- Re-run a specific migration manually:
  - `docker exec -it <db_container> psql -U postgres -d era -f /migrations/0001_inventory.sql`
- Verify tables exist:
  - `docker exec -it <db_container> psql -U postgres -d era -c "\dt"`

