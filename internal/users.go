package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// loginUser handles user authentication
func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Get user by email (without RLS - login is available to all users)
	query := `
		SELECT id, email, password_hash, first_name, last_name, org_id, roles, is_active, 
		       created_at, updated_at, last_login_at
		FROM users 
		WHERE email = $1 AND is_active = true`

	var user models.User
	var firstName, lastName sql.NullString
	var lastLoginAt sql.NullTime
	var roles pq.StringArray

	err := s.DB.QueryRow(query, req.Email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &firstName, &lastName,
		&user.OrgID, &roles, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Update last login time
	_, err = s.DB.Exec("UPDATE users SET last_login_at = now() WHERE id = $1", user.ID)
	if err != nil {
		// Log error but don't fail login
		fmt.Printf("Failed to update last_login_at: %v\n", err)
	}

	// Set optional fields
	if firstName.Valid {
		user.FirstName = &firstName.String
	}
	if lastName.Valid {
		user.LastName = &lastName.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	user.Roles = roles

	// Generate JWT token
	token, err := s.JWTManager.GenerateToken(user.ID, user.OrgID, user.Roles)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return response
	response := models.LoginResponse{
		Token: token,
		User:  user.Redacted(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// createUser handles user creation with multi-tenant logic
func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" || len(req.Roles) == 0 {
		http.Error(w, "Email, password, and roles are required", http.StatusBadRequest)
		return
	}

	// Validate roles
	if !models.ValidateRoles(req.Roles) {
		http.Error(w, "Invalid roles provided", http.StatusBadRequest)
		return
	}

	// Determine target organization
	targetOrgID := auth.GetTargetOrgID(r.Context(), req.OrgID)

	// Validate permissions
	if !auth.CanManageOrg(r.Context(), targetOrgID) {
		http.Error(w, "Cannot create users for this organization", http.StatusForbidden)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Insert user with RLS context
	conn, ctx, err := withDBConn(r.Context(), s.DB, auth.OrgIDFromContext(r.Context()))
	if err != nil {
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	query := `
		INSERT INTO users (email, password_hash, first_name, last_name, org_id, roles)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	var userID int64
	var createdAt, updatedAt time.Time

	err = conn.QueryRowContext(ctx, query,
		req.Email, string(hashedPassword), req.FirstName, req.LastName,
		targetOrgID, pq.Array(req.Roles)).Scan(&userID, &createdAt, &updatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "User with this email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Return created user
	user := models.User{
		ID:        userID,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		OrgID:     targetOrgID,
		Roles:     req.Roles,
		IsActive:  true,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// listUsers handles user listing with multi-tenant filtering
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	// Optional org filter for main tenant
	orgFilter := r.URL.Query().Get("org_id")

	query := `
		SELECT id, email, first_name, last_name, org_id, roles, is_active, 
		       created_at, updated_at, last_login_at
		FROM users`

	args := []interface{}{}

	// Add org filter if specified and user is main tenant
	if orgFilter != "" && auth.IsMainTenant(r.Context()) {
		orgID, err := strconv.ParseInt(orgFilter, 10, 64)
		if err != nil {
			http.Error(w, "Invalid org_id parameter", http.StatusBadRequest)
			return
		}
		query += " WHERE org_id = $1"
		args = append(args, orgID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var firstName, lastName sql.NullString
		var lastLoginAt sql.NullTime
		var roles pq.StringArray

		err := rows.Scan(
			&user.ID, &user.Email, &firstName, &lastName,
			&user.OrgID, &roles, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
		)
		if err != nil {
			http.Error(w, "Failed to scan user", http.StatusInternalServerError)
			return
		}

		// Set optional fields
		if firstName.Valid {
			user.FirstName = &firstName.String
		}
		if lastName.Valid {
			user.LastName = &lastName.String
		}
		if lastLoginAt.Valid {
			user.LastLoginAt = &lastLoginAt.Time
		}
		user.Roles = roles

		users = append(users, user.Redacted())
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// getUser handles getting a specific user
func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	query := `
		SELECT id, email, first_name, last_name, org_id, roles, is_active, 
		       created_at, updated_at, last_login_at
		FROM users 
		WHERE id = $1`

	var user models.User
	var firstName, lastName sql.NullString
	var lastLoginAt sql.NullTime
	var roles pq.StringArray

	err = s.DB.QueryRowContext(r.Context(), query, id).Scan(
		&user.ID, &user.Email, &firstName, &lastName,
		&user.OrgID, &roles, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Set optional fields
	if firstName.Valid {
		user.FirstName = &firstName.String
	}
	if lastName.Valid {
		user.LastName = &lastName.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	user.Roles = roles

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.Redacted())
}

// updateUser handles user updates with multi-tenant logic
func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get existing user first to check permissions
	var existingUser models.User
	query := `SELECT id, org_id FROM users WHERE id = $1`
	err = s.DB.QueryRowContext(r.Context(), query, id).Scan(&existingUser.ID, &existingUser.OrgID)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Validate permissions for org change
	if req.OrgID != nil && *req.OrgID != existingUser.OrgID {
		if !auth.IsMainTenant(r.Context()) {
			http.Error(w, "Only main tenant can change user organization", http.StatusForbidden)
			return
		}
	}

	// Validate roles if provided
	if req.Roles != nil && !models.ValidateRoles(req.Roles) {
		http.Error(w, "Invalid roles provided", http.StatusBadRequest)
		return
	}

	// Build update query dynamically
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.FirstName != nil {
		setParts = append(setParts, fmt.Sprintf("first_name = $%d", argIndex))
		args = append(args, req.FirstName)
		argIndex++
	}

	if req.LastName != nil {
		setParts = append(setParts, fmt.Sprintf("last_name = $%d", argIndex))
		args = append(args, req.LastName)
		argIndex++
	}

	if req.OrgID != nil {
		setParts = append(setParts, fmt.Sprintf("org_id = $%d", argIndex))
		args = append(args, *req.OrgID)
		argIndex++
	}

	if req.Roles != nil {
		setParts = append(setParts, fmt.Sprintf("roles = $%d", argIndex))
		args = append(args, pq.Array(req.Roles))
		argIndex++
	}

	if req.IsActive != nil {
		setParts = append(setParts, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}

	if len(setParts) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	setParts = append(setParts, "updated_at = now()")
	updateQuery := fmt.Sprintf(`
		UPDATE users 
		SET %s 
		WHERE id = $%d
		RETURNING id, email, first_name, last_name, org_id, roles, is_active, created_at, updated_at, last_login_at`,
		strings.Join(setParts, ", "), argIndex)

	args = append(args, id)

	var user models.User
	var firstName, lastName sql.NullString
	var lastLoginAt sql.NullTime
	var roles pq.StringArray

	err = s.DB.QueryRowContext(r.Context(), updateQuery, args...).Scan(
		&user.ID, &user.Email, &firstName, &lastName,
		&user.OrgID, &roles, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
	)

	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	// Set optional fields
	if firstName.Valid {
		user.FirstName = &firstName.String
	}
	if lastName.Valid {
		user.LastName = &lastName.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	user.Roles = roles

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.Redacted())
}

// deleteUser handles user deletion
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user exists and get their info
	var orgID int64
	var roles pq.StringArray
	query := `SELECT org_id, roles FROM users WHERE id = $1`
	err = s.DB.QueryRowContext(r.Context(), query, id).Scan(&orgID, &roles)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if this is the last org_admin in the organization
	if containsRole(roles, "org_admin") {
		var adminCount int
		countQuery := `SELECT COUNT(*) FROM users WHERE org_id = $1 AND roles && ARRAY['org_admin'] AND is_active = true AND id != $2`
		err = s.DB.QueryRowContext(r.Context(), countQuery, orgID, id).Scan(&adminCount)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if adminCount == 0 {
			http.Error(w, "Cannot delete the last org_admin in organization", http.StatusBadRequest)
			return
		}
	}

	// Delete the user
	deleteQuery := `DELETE FROM users WHERE id = $1`
	result, err := s.DB.ExecContext(r.Context(), deleteQuery, id)
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getUserProfile handles getting current user's profile
func (s *Server) getUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	query := `
		SELECT id, email, first_name, last_name, org_id, roles, is_active, 
		       created_at, updated_at, last_login_at
		FROM users 
		WHERE id = $1`

	var user models.User
	var firstName, lastName sql.NullString
	var lastLoginAt sql.NullTime
	var roles pq.StringArray

	err := s.DB.QueryRowContext(r.Context(), query, userID).Scan(
		&user.ID, &user.Email, &firstName, &lastName,
		&user.OrgID, &roles, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Set optional fields
	if firstName.Valid {
		user.FirstName = &firstName.String
	}
	if lastName.Valid {
		user.LastName = &lastName.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	user.Roles = roles

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.Redacted())
}

// updateUserProfile handles updating current user's profile
func (s *Server) updateUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	var req models.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build update query dynamically
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.FirstName != nil {
		setParts = append(setParts, fmt.Sprintf("first_name = $%d", argIndex))
		args = append(args, req.FirstName)
		argIndex++
	}

	if req.LastName != nil {
		setParts = append(setParts, fmt.Sprintf("last_name = $%d", argIndex))
		args = append(args, req.LastName)
		argIndex++
	}

	if len(setParts) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	setParts = append(setParts, "updated_at = now()")
	updateQuery := fmt.Sprintf(`
		UPDATE users 
		SET %s 
		WHERE id = $%d
		RETURNING id, email, first_name, last_name, org_id, roles, is_active, created_at, updated_at, last_login_at`,
		strings.Join(setParts, ", "), argIndex)

	args = append(args, userID)

	var user models.User
	var firstName, lastName sql.NullString
	var lastLoginAt sql.NullTime
	var roles pq.StringArray

	err := s.DB.QueryRowContext(r.Context(), updateQuery, args...).Scan(
		&user.ID, &user.Email, &firstName, &lastName,
		&user.OrgID, &roles, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLoginAt,
	)

	if err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	// Set optional fields
	if firstName.Valid {
		user.FirstName = &firstName.String
	}
	if lastName.Valid {
		user.LastName = &lastName.String
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	user.Roles = roles

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.Redacted())
}

// changePassword handles password changes
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Current password and new password are required", http.StatusBadRequest)
		return
	}

	// Get current password hash
	var currentPasswordHash string
	query := `SELECT password_hash FROM users WHERE id = $1`
	err := s.DB.QueryRowContext(r.Context(), query, userID).Scan(&currentPasswordHash)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(currentPasswordHash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "Current password is incorrect", http.StatusBadRequest)
		return
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash new password", http.StatusInternalServerError)
		return
	}

	// Update password
	updateQuery := `UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`
	_, err = s.DB.ExecContext(r.Context(), updateQuery, string(newPasswordHash), userID)
	if err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper function to check if a role exists in a slice
func containsRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
