package models

import (
	"database/sql/driver"
	"encoding/json"
	"net"
	"time"
)

// Asset represents the core asset record
type Asset struct {
	ID        int64     `json:"id"`
	OrgID     int64     `json:"org_id"`
	SiteID    int64     `json:"site_id"`
	AssetType string    `json:"asset_type"`
	Name      *string   `json:"name,omitempty"`
	Vendor    *string   `json:"vendor,omitempty"`
	Model     *string   `json:"model,omitempty"`
	Serial    *string   `json:"serial,omitempty"`
	MgmtIP    *net.IP   `json:"mgmt_ip,omitempty"`
	Status    *string   `json:"status,omitempty"`
	Notes     *string   `json:"notes,omitempty"`
	Extras    JSONB     `json:"extras"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AssetSwitch represents switch-specific attributes
type AssetSwitch struct {
	AssetID    int64   `json:"asset_id"`
	PortsTotal *int    `json:"ports_total,omitempty"`
	POE        *bool   `json:"poe,omitempty"`
	UplinkInfo *string `json:"uplink_info,omitempty"`
	Firmware   *string `json:"firmware,omitempty"`
}

// AssetVLAN represents VLAN-specific attributes
type AssetVLAN struct {
	AssetID int64   `json:"asset_id"`
	VLANID  int     `json:"vlan_id"`
	Subnet  *string `json:"subnet,omitempty"` // CIDR string
	Gateway *net.IP `json:"gateway,omitempty"`
	Purpose *string `json:"purpose,omitempty"`
}

// SiteAssetCategory represents dynamic site categories
type SiteAssetCategory struct {
	OrgID      int64  `json:"org_id"`
	SiteID     int64  `json:"site_id"`
	AssetType  string `json:"asset_type"`
	AssetCount int    `json:"asset_count"`
}

// CreateAssetRequest represents the request body for creating a new asset
type CreateAssetRequest struct {
	SiteID    int64                  `json:"site_id" validate:"required"`
	AssetType string                 `json:"asset_type" validate:"required"`
	Name      *string                `json:"name,omitempty"`
	Vendor    *string                `json:"vendor,omitempty"`
	Model     *string                `json:"model,omitempty"`
	Serial    *string                `json:"serial,omitempty"`
	MgmtIP    *string                `json:"mgmt_ip,omitempty"`
	Status    *string                `json:"status,omitempty"`
	Notes     *string                `json:"notes,omitempty"`
	Extras    map[string]interface{} `json:"extras,omitempty"`
	// Optional subtype data
	Switch *CreateAssetSwitchRequest `json:"switch,omitempty"`
	VLAN   *CreateAssetVLANRequest   `json:"vlan,omitempty"`
}

// CreateAssetSwitchRequest represents switch-specific creation data
type CreateAssetSwitchRequest struct {
	PortsTotal *int    `json:"ports_total,omitempty"`
	POE        *bool   `json:"poe,omitempty"`
	UplinkInfo *string `json:"uplink_info,omitempty"`
	Firmware   *string `json:"firmware,omitempty"`
}

// CreateAssetVLANRequest represents VLAN-specific creation data
type CreateAssetVLANRequest struct {
	VLANID  int     `json:"vlan_id" validate:"required"`
	Subnet  *string `json:"subnet,omitempty"`
	Gateway *string `json:"gateway,omitempty"`
	Purpose *string `json:"purpose,omitempty"`
}

// UpdateAssetRequest represents the request body for updating an asset
type UpdateAssetRequest struct {
	AssetType *string                `json:"asset_type,omitempty"`
	Name      *string                `json:"name,omitempty"`
	Vendor    *string                `json:"vendor,omitempty"`
	Model     *string                `json:"model,omitempty"`
	Serial    *string                `json:"serial,omitempty"`
	MgmtIP    *string                `json:"mgmt_ip,omitempty"`
	Status    *string                `json:"status,omitempty"`
	Notes     *string                `json:"notes,omitempty"`
	Extras    map[string]interface{} `json:"extras,omitempty"`
	// Optional subtype data
	Switch *UpdateAssetSwitchRequest `json:"switch,omitempty"`
	VLAN   *UpdateAssetVLANRequest   `json:"vlan,omitempty"`
}

// UpdateAssetSwitchRequest represents switch-specific update data
type UpdateAssetSwitchRequest struct {
	PortsTotal *int    `json:"ports_total,omitempty"`
	POE        *bool   `json:"poe,omitempty"`
	UplinkInfo *string `json:"uplink_info,omitempty"`
	Firmware   *string `json:"firmware,omitempty"`
}

// UpdateAssetVLANRequest represents VLAN-specific update data
type UpdateAssetVLANRequest struct {
	VLANID  *int    `json:"vlan_id,omitempty"`
	Subnet  *string `json:"subnet,omitempty"`
	Gateway *string `json:"gateway,omitempty"`
	Purpose *string `json:"purpose,omitempty"`
}

// AssetWithSubtypes represents an asset with its subtype data
type AssetWithSubtypes struct {
	Asset
	Switch *AssetSwitch `json:"switch,omitempty"`
	VLAN   *AssetVLAN   `json:"vlan,omitempty"`
}

// JSONB is a custom type for JSONB fields
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, j)
}
