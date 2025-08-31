# Era Inventory API — Progress Report

_Last updated: {{set date when you edit}}_

## 1) Snapshot
**Milestone status**
- ✅ M1 — Migrations & Schema (Postgres + idempotent migrate job)
- ✅ M2 — CRUD for Items / Sites / Vendors / Projects
- ✅ M2.5 — RBAC scaffold (org ID in request context)
- ⏳ M3 — AuthN/Z (JWT) + optional Postgres RLS
- ⏳ M3.5 — OpenAPI + Docs
- ⏳ M4 — Tests + CI
- ⏳ M5 — Metrics, QoL (Makefile, seed), small reports
- ⏳ M6 — Inventory flows (stock movements, check-in/out)

## 2) Completed ✔
- [x] Docker Compose with healthchecks (`db`, `api`, `migrate`)
- [x] Migrations applied:
  - `inventory`, `sites`, `vendors`, `projects`, `schema_migrations`
- [x] FK links: `inventory.site_id/vendor_id/project_id` (+ indexes)
- [x] Seed strategy (seed.sql service or ad-hoc `psql`)
- [x] CRUD endpoints:
  - `items`, `sites`, `vendors`, `projects`
- [x] List ergonomics:
  - pagination (`limit/offset`), search (`q`), sorting (`sort`)
  - response envelope: `{ data, page { limit, offset, total } }`
- [x] RBAC scaffold: org ID injected via middleware/context

## 3) In Progress 🚧
- [ ] JWT auth middleware (validate HS256, extract `sub`, `org_id`, `roles`)
- [ ] Role checks on POST/PUT/DELETE (e.g., `org_admin`)
- [ ] (Optional) Postgres RLS with `app.current_org_id`
- [ ] OpenAPI spec + Swagger UI at `/docs`

## 4) Next Up 🎯
- [ ] CI (GitHub Actions): spin Postgres, run migrations, `go test ./...`
- [ ] Prometheus `/metrics` + request/DB error counters
- [ ] Makefile targets (`up`, `migrate`, `seed`, `logs`, `psql`, `test`)
- [ ] Quick reports: counts by site/vendor/project, aging, top items

## 5) Endpoint Checklist (current)
- Health: `GET /health` — ✅
- Items: `GET/POST/PUT/DELETE /items` — ✅
- Sites: `GET/POST/PUT/DELETE /sites` — ✅
- Vendors: `GET/POST/PUT/DELETE /vendors` — ✅
- Projects: `GET/POST/PUT/DELETE /projects` — ✅
- Auth: `Authorization: Bearer <JWT>` — ⏳
- Docs: `GET /docs` (Swagger) — ⏳
- Metrics: `GET /metrics` — ⏳

## 6) Ops & DB
- Compose: ✅ `depends_on.condition: service_healthy` for `api`/`migrate`
- DB indices: ✅ FK indices + (optional) pg_trgm for name search
- Backups: ⏳ doc `pg_dump` routine (nightly), restore procedure

## 7) Decisions Log (abridged)
- Go + Postgres 16 with `pgxpool`
- Idempotent SQL migrations via containerized runner
- Org-scoped RBAC first; RLS behind feature flag later
- REST first; OpenAPI + SDK gen later

## 8) How to Verify (quick commands)
```bash
docker compose up migrate
docker compose exec db psql -U postgres -d era -c "\dt"
docker compose logs -f api
