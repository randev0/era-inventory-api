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

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
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
		SELECT id, name, location, notes, created_at, updated_at
		FROM sites`
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

	sites := []models.Site{}
	for rows.Next() {
		var sc models.Site
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Location, &sc.Notes, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		sites = append(sites, sc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sites)
}

func (s *Server) getSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sc models.Site
	err := s.DB.QueryRow(`
		SELECT id, name, location, notes, created_at, updated_at
		FROM sites WHERE id = $1`, id).Scan(&sc.ID, &sc.Name, &sc.Location, &sc.Notes, &sc.CreatedAt, &sc.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sc)
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var in models.Site
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		http.Error(w, "name is required", 400)
		return
	}

	err := s.DB.QueryRow(`
		INSERT INTO sites (name, location, notes)
		VALUES ($1,$2,$3)
		RETURNING id, name, location, notes, created_at, updated_at
	`, in.Name, nullIfEmpty(in.Location), nullIfEmpty(in.Notes)).Scan(&in.ID, &in.Name, &in.Location, &in.Notes, &in.CreatedAt, &in.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(in)
}

func (s *Server) updateSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in models.Site
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	type set struct {
		sql string
		val interface{}
	}
	sets := make([]set, 0, 5)
	if strings.TrimSpace(in.Name) != "" {
		sets = append(sets, set{"name = $%d", in.Name})
	}
	if in.Location != nil {
		sets = append(sets, set{"location = $%d", nullIfEmpty(in.Location)})
	}
	if in.Notes != nil {
		sets = append(sets, set{"notes = $%d", nullIfEmpty(in.Notes)})
	}
	if len(sets) == 0 {
		http.Error(w, "no fields to update", 400)
		return
	}

	args := make([]interface{}, 0, len(sets)+1)
	sqlStr := "UPDATE sites SET "
	for i, sset := range sets {
		if i > 0 {
			sqlStr += ", "
		}
		sqlStr += fmt.Sprintf(sset.sql, i+1)
		args = append(args, sset.val)
	}
	sqlStr += fmt.Sprintf(" WHERE id = $%d RETURNING id, name, location, notes, created_at, updated_at", len(args)+1)
	args = append(args, id)

	var out models.Site
	if err := s.DB.QueryRow(sqlStr, args...).Scan(&out.ID, &out.Name, &out.Location, &out.Notes, &out.CreatedAt, &out.UpdatedAt); err != nil {
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

func (s *Server) deleteSite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.DB.Exec(`DELETE FROM sites WHERE id = $1`, id)
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

// nullIfEmpty converts empty string pointer to nil for nullable columns
func nullIfEmpty(s *string) interface{} {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(*s) == "" {
		return nil
	}
	return *s
}

