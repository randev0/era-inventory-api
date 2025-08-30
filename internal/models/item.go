package models

import "time"

type Item struct {
	ID           int        `json:"id"`
	AssetTag     string     `json:"asset_tag"`
	Name         string     `json:"name"`
	Manufacturer string     `json:"manufacturer,omitempty"`
	Model        string     `json:"model,omitempty"`
	DeviceType   string     `json:"device_type,omitempty"`
	Site         string     `json:"site,omitempty"`
	InstalledAt  *time.Time `json:"installed_at,omitempty"`
	WarrantyEnd  *time.Time `json:"warranty_end,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
