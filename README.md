# Era Inventory API

A simple **Go + Postgres REST API** for managing an inventory system.  
Built with [Go Chi](https://github.com/go-chi/chi) for routing, [pgx](https://github.com/jackc/pgx) for PostgreSQL, and fully containerized with Docker.

---

## 🚀 Features

- Health checks (`/health`, `/dbping`)
- Full CRUD for inventory items:
  - `POST   /items` → create
  - `GET    /items` → list with pagination & filters
  - `GET    /items/{id}` → fetch one
  - `PUT    /items/{id}` → update (partial allowed)
  - `DELETE /items/{id}` → remove
- Filters: search by query, type, site
- Pagination (`page`, `limit` params)
- Unique `asset_tag` constraint
- JSON responses, ready for frontend integration
- Dockerized with `docker-compose`

---

## 📂 Project Structure

