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

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)

	clauses := []string{}
	args := []interface{}{}
	arg := 1

	// org filter
	clauses = append(clauses, fmt.Sprintf("org_id = $%d", arg))
	args = append(args, params.orgID)
	arg++

	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("(code ILIKE $%d OR name ILIKE $%d)", arg, arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	sqlStr := `
		SELECT id, code, name, description, created_at, updated_at
		FROM projects`
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

	projects := []models.Project{}
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		projects = append(projects, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p models.Project
	err := s.DB.QueryRow(`
		SELECT id, code, name, description, created_at, updated_at
		FROM projects WHERE id = $1`, id).Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
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

	err := s.DB.QueryRow(`
		INSERT INTO projects (code, name, description)
		VALUES ($1,$2,$3)
		RETURNING id, code, name, description, created_at, updated_at
	`, in.Code, in.Name, nullIfEmpty(in.Description)).Scan(&in.ID, &in.Code, &in.Name, &in.Description, &in.CreatedAt, &in.UpdatedAt)
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
	json.NewEncoder(w).Encode(in)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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

	args := make([]interface{}, 0, len(sets)+1)
	sqlStr := "UPDATE projects SET "
	for i, sset := range sets {
		if i > 0 {
			sqlStr += ", "
		}
		sqlStr += fmt.Sprintf(sset.sql, i+1)
		args = append(args, sset.val)
	}
	sqlStr += fmt.Sprintf(" WHERE id = $%d RETURNING id, code, name, description, created_at, updated_at", len(args)+1)
	args = append(args, id)

	var out models.Project
	if err := s.DB.QueryRow(sqlStr, args...).Scan(&out.ID, &out.Code, &out.Name, &out.Description, &out.CreatedAt, &out.UpdatedAt); err != nil {
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
	json.NewEncoder(w).Encode(out)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.DB.Exec(`DELETE FROM projects WHERE id = $1`, id)
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

