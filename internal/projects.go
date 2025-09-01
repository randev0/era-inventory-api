package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/models"

	"github.com/go-chi/chi/v5"
)

// LIST with basic filters & pagination
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)
	orgID := auth.OrgIDFromContext(r.Context())

	clauses := []string{}
	args := []interface{}{}
	arg := 1

	// org filter - use context value instead of query param
	clauses = append(clauses, fmt.Sprintf("org_id = $%d", arg))
	args = append(args, orgID)
	arg++

	// optional text search on name
	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("(code ILIKE $%d OR name ILIKE $%d)", arg, arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	whereClause := ""
	if len(clauses) > 0 {
		whereClause = " WHERE " + strings.Join(clauses, " AND ")
	}

	// Build the main query with COUNT(*) OVER() to get total count
	sqlStr := fmt.Sprintf(`
		SELECT id, code, name, description, created_at, updated_at,
		       COUNT(*) OVER() as total_count
		FROM projects%s`, whereClause)

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

	projects := []interface{}{}
	var totalCount int
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &totalCount); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		projects = append(projects, p)
	}

	sendListResponse(w, projects, totalCount, params)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	var p models.Project
	q := dbFrom(r.Context(), s.DB)
	err := q.QueryRowContext(r.Context(), `
		SELECT id, code, name, description, created_at, updated_at
		FROM projects WHERE id = $1 AND org_id = $2`, id, orgID).Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var in models.Project
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" {
		http.Error(w, "code and name are required", 400)
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())

	q := dbFrom(r.Context(), s.DB)
	err := q.QueryRowContext(r.Context(), `
		INSERT INTO projects (code, name, description, org_id)
		VALUES ($1,$2,$3,$4)
		RETURNING id, code, name, description, created_at, updated_at
	`, in.Code, in.Name, nullIfEmpty(in.Description), orgID).Scan(&in.ID, &in.Code, &in.Name, &in.Description, &in.CreatedAt, &in.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "code already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	var in models.Project
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	type set struct {
		sql string
		val interface{}
	}
	sets := make([]set, 0, 5)
	if strings.TrimSpace(in.Code) != "" {
		sets = append(sets, set{"code = $%d", in.Code})
	}
	if strings.TrimSpace(in.Name) != "" {
		sets = append(sets, set{"name = $%d", in.Name})
	}
	if in.Description != nil {
		sets = append(sets, set{"description = $%d", nullIfEmpty(in.Description)})
	}
	if len(sets) == 0 {
		http.Error(w, "no fields to update", 400)
		return
	}

	args := make([]interface{}, 0, len(sets)+2)
	sqlStr := "UPDATE projects SET "
	for i, sset := range sets {
		if i > 0 {
			sqlStr += ", "
		}
		sqlStr += fmt.Sprintf(sset.sql, i+1)
		args = append(args, sset.val)
	}
	sqlStr += fmt.Sprintf(" WHERE id = $%d AND org_id = $%d RETURNING id, code, name, description, created_at, updated_at", len(args)+1, len(args)+2)
	args = append(args, id, orgID)

	q := dbFrom(r.Context(), s.DB)
	var out models.Project
	if err := q.QueryRowContext(r.Context(), sqlStr, args...).Scan(&out.ID, &out.Code, &out.Name, &out.Description, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "code already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	q := dbFrom(r.Context(), s.DB)
	res, err := q.ExecContext(r.Context(), `DELETE FROM projects WHERE id = $1 AND org_id = $2`, id, orgID)
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
