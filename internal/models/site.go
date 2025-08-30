package models

import "time"

type Site struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Location  *string   `json:"location,omitempty"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

