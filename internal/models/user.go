package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never expose in JSON
	FirstName    *string    `json:"first_name,omitempty"`
	LastName     *string    `json:"last_name,omitempty"`
	OrgID        int64      `json:"org_id"`
	Roles        []string   `json:"roles"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// CreateUserRequest represents the request body for creating a new user
type CreateUserRequest struct {
	Email     string   `json:"email" validate:"required,email"`
	Password  string   `json:"password" validate:"required,min=8"`
	FirstName *string  `json:"first_name,omitempty"`
	LastName  *string  `json:"last_name,omitempty"`
	OrgID     *int64   `json:"org_id,omitempty"` // Optional: main tenant can specify, others use their own
	Roles     []string `json:"roles" validate:"required,min=1"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	FirstName *string  `json:"first_name,omitempty"`
	LastName  *string  `json:"last_name,omitempty"`
	OrgID     *int64   `json:"org_id,omitempty"` // Only main tenant can change this
	Roles     []string `json:"roles,omitempty"`
	IsActive  *bool    `json:"is_active,omitempty"`
}

// LoginRequest represents the request body for user login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the response body for successful login
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// ChangePasswordRequest represents the request body for changing password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// UpdateProfileRequest represents the request body for updating user profile
type UpdateProfileRequest struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
}

// Organization represents an organization in the system
type Organization struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateOrganizationRequest represents the request body for creating a new organization
type CreateOrganizationRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// ValidRoles defines the available roles in the system
var ValidRoles = []string{
	"viewer",
	"project_admin",
	"org_admin",
}

// IsValidRole checks if a role is valid
func IsValidRole(role string) bool {
	for _, validRole := range ValidRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

// ValidateRoles checks if all provided roles are valid
func ValidateRoles(roles []string) bool {
	for _, role := range roles {
		if !IsValidRole(role) {
			return false
		}
	}
	return len(roles) > 0
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(role string) bool {
	for _, userRole := range u.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (u *User) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// IsMainTenant checks if the user belongs to the main tenant (org_id = 1)
func (u *User) IsMainTenant() bool {
	return u.OrgID == 1
}

// CanManageOrg checks if the user can manage the specified organization
func (u *User) CanManageOrg(targetOrgID int64) bool {
	// Main tenant with org_admin can manage any org
	if u.IsMainTenant() && u.HasRole("org_admin") {
		return true
	}
	// Other users can only manage their own org
	return u.OrgID == targetOrgID && u.HasRole("org_admin")
}

// GetDisplayName returns the user's display name
func (u *User) GetDisplayName() string {
	if u.FirstName != nil && u.LastName != nil {
		return *u.FirstName + " " + *u.LastName
	}
	if u.FirstName != nil {
		return *u.FirstName
	}
	if u.LastName != nil {
		return *u.LastName
	}
	return u.Email
}

// Redacted returns a copy of the user with sensitive fields removed
func (u *User) Redacted() User {
	return User{
		ID:          u.ID,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		OrgID:       u.OrgID,
		Roles:       u.Roles,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}
