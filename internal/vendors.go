package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"era-inventory-api/internal/models"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listVendors(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)

	clauses := []string{}
	args := []interface{}{}
	arg := 1

	// org filter
	clauses = append(clauses, fmt.Sprintf("org_id = $%d", arg))
	args = append(args, params.orgID)
	arg++

	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("name ILIKE $%d", arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	sqlStr := `
		SELECT id, name, email, phone, notes, created_at, updated_at
		FROM vendors`
	if len(clauses) > 0 {
		sqlStr += " WHERE " + strings.Join(clauses, " AND ")
	}

	allowedSort := map[string]string{
		"id":         "id",
		"name":       "name",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	sqlStr += buildOrderBy(params.sort, allowedSort)
	sqlStr += fmt.Sprintf(" LIMIT %d OFFSET %d", params.limit, params.offset)

	rows, err := s.DB.Query(sqlStr, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	vendors := []models.Vendor{}
	for rows.Next() {
		var v models.Vendor
		if err := rows.Scan(&v.ID, &v.Name, &v.Email, &v.Phone, &v.Notes, &v.CreatedAt, &v.UpdatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		vendors = append(vendors, v)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vendors)
}

func (s *Server) getVendor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var v models.Vendor
	err := s.DB.QueryRow(`
		SELECT id, name, email, phone, notes, created_at, updated_at
		FROM vendors WHERE id = $1`, id).Scan(&v.ID, &v.Name, &v.Email, &v.Phone, &v.Notes, &v.CreatedAt, &v.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) createVendor(w http.ResponseWriter, r *http.Request) {
	var in models.Vendor
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		http.Error(w, "name is required", 400)
		return
	}

	err := s.DB.QueryRow(`
		INSERT INTO vendors (name, email, phone, notes)
		VALUES ($1,$2,$3,$4)
		RETURNING id, name, email, phone, notes, created_at, updated_at
	`, in.Name, nullIfEmpty(in.Email), nullIfEmpty(in.Phone), nullIfEmpty(in.Notes)).Scan(&in.ID, &in.Name, &in.Email, &in.Phone, &in.Notes, &in.CreatedAt, &in.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(in)
}

func (s *Server) updateVendor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in models.Vendor
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	type set struct {
		sql string
		val interface{}
	}
	sets := make([]set, 0, 6)
	if strings.TrimSpace(in.Name) != "" {
		sets = append(sets, set{"name = $%d", in.Name})
	}
	if in.Email != nil {
		sets = append(sets, set{"email = $%d", nullIfEmpty(in.Email)})
	}
	if in.Phone != nil {
		sets = append(sets, set{"phone = $%d", nullIfEmpty(in.Phone)})
	}
	if in.Notes != nil {
		sets = append(sets, set{"notes = $%d", nullIfEmpty(in.Notes)})
	}
	if len(sets) == 0 {
		http.Error(w, "no fields to update", 400)
		return
	}

	args := make([]interface{}, 0, len(sets)+1)
	sqlStr := "UPDATE vendors SET "
	for i, sset := range sets {
		if i > 0 {
			sqlStr += ", "
		}
		sqlStr += fmt.Sprintf(sset.sql, i+1)
		args = append(args, sset.val)
	}
	sqlStr += fmt.Sprintf(" WHERE id = $%d RETURNING id, name, email, phone, notes, created_at, updated_at", len(args)+1)
	args = append(args, id)

	var out models.Vendor
	if err := s.DB.QueryRow(sqlStr, args...).Scan(&out.ID, &out.Name, &out.Email, &out.Phone, &out.Notes, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) deleteVendor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.DB.Exec(`DELETE FROM vendors WHERE id = $1`, id)
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

