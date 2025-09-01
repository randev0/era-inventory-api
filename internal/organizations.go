package internal

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/models"

	"github.com/go-chi/chi/v5"
)

// listOrganizations handles listing all organizations (main tenant only)
func (s *Server) listOrganizations(w http.ResponseWriter, r *http.Request) {
	// Only main tenant can access organizations
	if !auth.IsMainTenant(r.Context()) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	query := `
		SELECT id, name, created_at, updated_at
		FROM organizations
		ORDER BY name`

	rows, err := s.DB.QueryContext(r.Context(), query)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var organizations []models.Organization
	for rows.Next() {
		var org models.Organization
		err := rows.Scan(&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt)
		if err != nil {
			http.Error(w, "Failed to scan organization", http.StatusInternalServerError)
			return
		}
		organizations = append(organizations, org)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organizations)
}

// getOrganization handles getting a specific organization (main tenant only)
func (s *Server) getOrganization(w http.ResponseWriter, r *http.Request) {
	// Only main tenant can access organizations
	if !auth.IsMainTenant(r.Context()) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	orgID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(orgID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	query := `
		SELECT id, name, created_at, updated_at
		FROM organizations
		WHERE id = $1`

	var org models.Organization
	err = s.DB.QueryRowContext(r.Context(), query, id).Scan(
		&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

// createOrganization handles creating a new organization (main tenant only)
func (s *Server) createOrganization(w http.ResponseWriter, r *http.Request) {
	// Only main tenant can create organizations
	if !auth.IsMainTenant(r.Context()) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	var req models.CreateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" {
		http.Error(w, "Organization name is required", http.StatusBadRequest)
		return
	}

	// Insert organization
	query := `
		INSERT INTO organizations (name)
		VALUES ($1)
		RETURNING id, created_at, updated_at`

	var org models.Organization
	err := s.DB.QueryRowContext(r.Context(), query, req.Name).Scan(
		&org.ID, &org.CreatedAt, &org.UpdatedAt,
	)

	if err != nil {
		if err.Error() == `pq: duplicate key value violates unique constraint "organizations_name_key"` {
			http.Error(w, "Organization with this name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create organization", http.StatusInternalServerError)
		return
	}

	org.Name = req.Name

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(org)
}

// updateOrganization handles updating an organization (main tenant only)
func (s *Server) updateOrganization(w http.ResponseWriter, r *http.Request) {
	// Only main tenant can update organizations
	if !auth.IsMainTenant(r.Context()) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	orgID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(orgID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	var req models.CreateOrganizationRequest // Same structure for update
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" {
		http.Error(w, "Organization name is required", http.StatusBadRequest)
		return
	}

	// Update organization
	query := `
		UPDATE organizations 
		SET name = $1, updated_at = now()
		WHERE id = $2
		RETURNING id, name, created_at, updated_at`

	var org models.Organization
	err = s.DB.QueryRowContext(r.Context(), query, req.Name, id).Scan(
		&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		if err.Error() == `pq: duplicate key value violates unique constraint "organizations_name_key"` {
			http.Error(w, "Organization with this name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to update organization", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

// deleteOrganization handles deleting an organization (main tenant only)
func (s *Server) deleteOrganization(w http.ResponseWriter, r *http.Request) {
	// Only main tenant can delete organizations
	if !auth.IsMainTenant(r.Context()) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	orgID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(orgID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Prevent deleting main tenant
	if id == 1 {
		http.Error(w, "Cannot delete main tenant organization", http.StatusBadRequest)
		return
	}

	// Check if organization has users
	var userCount int
	countQuery := `SELECT COUNT(*) FROM users WHERE org_id = $1`
	err = s.DB.QueryRowContext(r.Context(), countQuery, id).Scan(&userCount)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if userCount > 0 {
		http.Error(w, "Cannot delete organization with existing users", http.StatusBadRequest)
		return
	}

	// Check if organization has other data (sites, vendors, projects, inventory)
	tables := []string{"sites", "vendors", "projects", "inventory"}
	for _, table := range tables {
		var dataCount int
		query := `SELECT COUNT(*) FROM ` + table + ` WHERE org_id = $1`
		err = s.DB.QueryRowContext(r.Context(), query, id).Scan(&dataCount)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if dataCount > 0 {
			http.Error(w, "Cannot delete organization with existing data", http.StatusBadRequest)
			return
		}
	}

	// Delete the organization
	deleteQuery := `DELETE FROM organizations WHERE id = $1`
	result, err := s.DB.ExecContext(r.Context(), deleteQuery, id)
	if err != nil {
		http.Error(w, "Failed to delete organization", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getOrganizationStats returns statistics about an organization (main tenant only)
func (s *Server) getOrganizationStats(w http.ResponseWriter, r *http.Request) {
	// Only main tenant can access organization stats
	if !auth.IsMainTenant(r.Context()) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	orgID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(orgID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Get organization details
	var org models.Organization
	orgQuery := `SELECT id, name, created_at, updated_at FROM organizations WHERE id = $1`
	err = s.DB.QueryRowContext(r.Context(), orgQuery, id).Scan(
		&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get counts for each entity type
	type Stats struct {
		Organization models.Organization `json:"organization"`
		Users        int                 `json:"users"`
		Sites        int                 `json:"sites"`
		Vendors      int                 `json:"vendors"`
		Projects     int                 `json:"projects"`
		Items        int                 `json:"items"`
	}

	var stats Stats
	stats.Organization = org

	// Count users
	err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE org_id = $1", id).Scan(&stats.Users)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Count sites
	err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM sites WHERE org_id = $1", id).Scan(&stats.Sites)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Count vendors
	err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM vendors WHERE org_id = $1", id).Scan(&stats.Vendors)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Count projects
	err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM projects WHERE org_id = $1", id).Scan(&stats.Projects)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Count inventory items
	err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM inventory WHERE org_id = $1", id).Scan(&stats.Items)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
