package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"era-inventory-api/internal/models"

	"era-inventory-api/internal/auth"

	"github.com/go-chi/chi/v5"
)

// LIST with basic filters & pagination
func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)
	orgID := auth.OrgIDFromContext(r.Context())

	clauses := []string{}
	args := []interface{}{}
	arg := 1

	// org filter - use context value instead of query param
	clauses = append(clauses, fmt.Sprintf("org_id = $%d", arg))
	args = append(args, orgID)
	arg++

	// optional text search on name/code/sku/serial → map to name or asset_tag
	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("(name ILIKE $%d OR asset_tag ILIKE $%d)", arg, arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	whereClause := ""
	if len(clauses) > 0 {
		whereClause = " WHERE " + strings.Join(clauses, " AND ")
	}

	// Build the main query with COUNT(*) OVER() to get total count
	sqlStr := fmt.Sprintf(`
		SELECT id, asset_tag, name, manufacturer, model, device_type, site,
		       installed_at, warranty_end, notes, created_at, updated_at,
		       COUNT(*) OVER() as total_count
		FROM inventory%s`, whereClause)

	allowedSort := map[string]string{
		"id":         "id",
		"name":       "name",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	sqlStr += buildOrderBy(params.sort, allowedSort)
	sqlStr += fmt.Sprintf(" LIMIT %d OFFSET %d", params.limit, params.offset)

	q := dbFrom(r.Context(), s.DB)
	rows, err := q.QueryContext(r.Context(), sqlStr, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	items := []interface{}{}
	var totalCount int
	for rows.Next() {
		var it models.Item
		if err := rows.Scan(
			&it.ID, &it.AssetTag, &it.Name, &it.Manufacturer, &it.Model, &it.DeviceType,
			&it.Site, &it.InstalledAt, &it.WarrantyEnd, &it.Notes, &it.CreatedAt, &it.UpdatedAt,
			&totalCount,
		); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		items = append(items, it)
	}

	sendListResponse(w, items, totalCount, params)
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	var it models.Item
	q := dbFrom(r.Context(), s.DB)
	err := q.QueryRowContext(r.Context(), `
		SELECT id, asset_tag, name, manufacturer, model, device_type, site,
		       installed_at, warranty_end, notes, created_at, updated_at
		FROM inventory WHERE id = $1 AND org_id = $2`, id, orgID).Scan(
		&it.ID, &it.AssetTag, &it.Name, &it.Manufacturer, &it.Model, &it.DeviceType,
		&it.Site, &it.InstalledAt, &it.WarrantyEnd, &it.Notes, &it.CreatedAt, &it.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(it)
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var in models.Item
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if in.AssetTag == "" || in.Name == "" {
		http.Error(w, "asset_tag and name are required", 400)
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())

	q := dbFrom(r.Context(), s.DB)
	err := q.QueryRowContext(r.Context(), `
		INSERT INTO inventory (asset_tag, name, manufacturer, model, device_type, site, installed_at, warranty_end, notes, org_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, created_at, updated_at
	`, in.AssetTag, in.Name, in.Manufacturer, in.Model, in.DeviceType, in.Site, in.InstalledAt, in.WarrantyEnd, in.Notes, orgID).
		Scan(&in.ID, &in.CreatedAt, &in.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "inventory_asset_tag_key") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "asset_tag already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(in)
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	var in models.Item
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	type set struct {
		sql string
		val interface{}
	}
	sets := make([]set, 0, 10)
	if in.AssetTag != "" {
		sets = append(sets, set{"asset_tag = $%d", in.AssetTag})
	}
	if in.Name != "" {
		sets = append(sets, set{"name = $%d", in.Name})
	}
	if in.Manufacturer != "" {
		sets = append(sets, set{"manufacturer = $%d", in.Manufacturer})
	}
	if in.Model != "" {
		sets = append(sets, set{"model = $%d", in.Model})
	}
	if in.DeviceType != "" {
		sets = append(sets, set{"device_type = $%d", in.DeviceType})
	}
	if in.Site != "" {
		sets = append(sets, set{"site = $%d", in.Site})
	}
	if in.InstalledAt != nil {
		sets = append(sets, set{"installed_at = $%d", in.InstalledAt})
	}
	if in.WarrantyEnd != nil {
		sets = append(sets, set{"warranty_end = $%d", in.WarrantyEnd})
	}
	if in.Notes != "" {
		sets = append(sets, set{"notes = $%d", in.Notes})
	}
	if len(sets) == 0 {
		http.Error(w, "no fields to update", 400)
		return
	}

	args := make([]interface{}, 0, len(sets)+2)
	sqlStr := "UPDATE inventory SET "
	for i, sset := range sets {
		if i > 0 {
			sqlStr += ", "
		}
		sqlStr += fmt.Sprintf(sset.sql, i+1)
		args = append(args, sset.val)
	}
	sqlStr += fmt.Sprintf(" WHERE id = $%d AND org_id = $%d RETURNING id, asset_tag, name, manufacturer, model, device_type, site, installed_at, warranty_end, notes, created_at, updated_at", len(args)+1, len(args)+2)
	args = append(args, id, orgID)

	q := dbFrom(r.Context(), s.DB)
	var out models.Item
	if err := q.QueryRowContext(r.Context(), sqlStr, args...).Scan(
		&out.ID, &out.AssetTag, &out.Name, &out.Manufacturer, &out.Model, &out.DeviceType,
		&out.Site, &out.InstalledAt, &out.WarrantyEnd, &out.Notes, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "inventory_asset_tag_key") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "asset_tag already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	q := dbFrom(r.Context(), s.DB)
	res, err := q.ExecContext(r.Context(), `DELETE FROM inventory WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
