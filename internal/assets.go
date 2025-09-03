package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/models"

	"github.com/go-chi/chi/v5"
)

// listAssets handles asset listing with filters and pagination
func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)
	orgID := auth.OrgIDFromContext(r.Context())

	// Cap limit at 100 as specified in requirements
	if params.limit > 100 {
		params.limit = 100
	}

	clauses := []string{}
	args := []interface{}{}
	arg := 1

	// org filter - use context value instead of query param
	clauses = append(clauses, fmt.Sprintf("org_id = $%d", arg))
	args = append(args, orgID)
	arg++

	// optional site filter
	if siteIDStr := strings.TrimSpace(r.URL.Query().Get("site_id")); siteIDStr != "" {
		if siteID, err := strconv.ParseInt(siteIDStr, 10, 64); err == nil {
			clauses = append(clauses, fmt.Sprintf("site_id = $%d", arg))
			args = append(args, siteID)
			arg++
		}
	}

	// optional type filter
	if assetType := strings.TrimSpace(r.URL.Query().Get("type")); assetType != "" {
		clauses = append(clauses, fmt.Sprintf("asset_type = $%d", arg))
		args = append(args, assetType)
		arg++
	}

	// optional text search on name
	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("name ILIKE $%d", arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	whereClause := ""
	if len(clauses) > 0 {
		whereClause = " WHERE " + strings.Join(clauses, " AND ")
	}

	// Build the main query with COUNT(*) OVER() to get total count
	sqlStr := fmt.Sprintf(`
		SELECT id, org_id, site_id, asset_type, name, vendor, model, serial, mgmt_ip, status, notes, extras, created_at, updated_at,
		       COUNT(*) OVER() as total_count
		FROM assets%s`, whereClause)

	allowedSort := map[string]string{
		"id":         "id",
		"name":       "name",
		"asset_type": "asset_type",
		"vendor":     "vendor",
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

	assets := []interface{}{}
	var totalCount int
	for rows.Next() {
		var a models.Asset
		var mgmtIPStr *string
		var extrasJSON []byte
		if err := rows.Scan(&a.ID, &a.OrgID, &a.SiteID, &a.AssetType, &a.Name, &a.Vendor, &a.Model, &a.Serial, &mgmtIPStr, &a.Status, &a.Notes, &extrasJSON, &a.CreatedAt, &a.UpdatedAt, &totalCount); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Parse mgmt_ip
		if mgmtIPStr != nil {
			if ip := net.ParseIP(*mgmtIPStr); ip != nil {
				a.MgmtIP = &ip
			}
		}

		// Parse extras JSON
		if len(extrasJSON) > 0 {
			if err := json.Unmarshal(extrasJSON, &a.Extras); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}

		assets = append(assets, a)
	}

	sendListResponse(w, assets, totalCount, params)
}

// getAsset handles getting a single asset by ID
func (s *Server) getAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	var a models.Asset
	var mgmtIPStr *string
	var extrasJSON []byte
	q := dbFrom(r.Context(), s.DB)
	err := q.QueryRowContext(r.Context(), `
		SELECT id, org_id, site_id, asset_type, name, vendor, model, serial, mgmt_ip, status, notes, extras, created_at, updated_at
		FROM assets WHERE id = $1 AND org_id = $2`, id, orgID).Scan(&a.ID, &a.OrgID, &a.SiteID, &a.AssetType, &a.Name, &a.Vendor, &a.Model, &a.Serial, &mgmtIPStr, &a.Status, &a.Notes, &extrasJSON, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Parse mgmt_ip
	if mgmtIPStr != nil {
		if ip := net.ParseIP(*mgmtIPStr); ip != nil {
			a.MgmtIP = &ip
		}
	}

	// Parse extras JSON
	if len(extrasJSON) > 0 {
		if err := json.Unmarshal(extrasJSON, &a.Extras); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(a); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// createAsset handles creating a new asset
func (s *Server) createAsset(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	if req.SiteID == 0 || req.AssetType == "" {
		http.Error(w, "site_id and asset_type are required", 400)
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())

	// Parse mgmt_ip if provided
	var mgmtIP interface{}
	if req.MgmtIP != nil {
		if ip := net.ParseIP(*req.MgmtIP); ip != nil {
			mgmtIP = ip.String()
		} else {
			http.Error(w, "invalid mgmt_ip format", 400)
			return
		}
	}

	// Convert extras to JSONB
	var extrasJSON []byte
	if req.Extras != nil {
		var err error
		extrasJSON, err = json.Marshal(req.Extras)
		if err != nil {
			http.Error(w, "invalid extras JSON", 400)
			return
		}
	} else {
		extrasJSON = []byte("{}")
	}

	q := dbFrom(r.Context(), s.DB)
	var assetID int64
	var createdAt, updatedAt string
	err := q.QueryRowContext(r.Context(), `
		INSERT INTO assets (org_id, site_id, asset_type, name, vendor, model, serial, mgmt_ip, status, notes, extras)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`, orgID, req.SiteID, req.AssetType, req.Name, req.Vendor, req.Model, req.Serial, mgmtIP, req.Status, req.Notes, extrasJSON).
		Scan(&assetID, &createdAt, &updatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "asset with this serial already exists for this site and type", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}

	// Create subtype records if provided
	if req.Switch != nil {
		_, err = q.ExecContext(r.Context(), `
			INSERT INTO asset_switches (asset_id, ports_total, poe, uplink_info, firmware)
			VALUES ($1, $2, $3, $4, $5)
		`, assetID, req.Switch.PortsTotal, req.Switch.POE, req.Switch.UplinkInfo, req.Switch.Firmware)
		if err != nil {
			http.Error(w, "failed to create switch subtype: "+err.Error(), 500)
			return
		}
	}

	if req.VLAN != nil {
		_, err = q.ExecContext(r.Context(), `
			INSERT INTO asset_vlans (asset_id, vlan_id, subnet, gateway, purpose)
			VALUES ($1, $2, $3, $4, $5)
		`, assetID, req.VLAN.VLANID, req.VLAN.Subnet, req.VLAN.Gateway, req.VLAN.Purpose)
		if err != nil {
			http.Error(w, "failed to create VLAN subtype: "+err.Error(), 500)
			return
		}
	}

	// Return created asset
	asset := models.Asset{
		ID:        assetID,
		OrgID:     orgID,
		SiteID:    req.SiteID,
		AssetType: req.AssetType,
		Name:      req.Name,
		Vendor:    req.Vendor,
		Model:     req.Model,
		Serial:    req.Serial,
		Status:    req.Status,
		Notes:     req.Notes,
		Extras:    models.JSONB(req.Extras),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(asset); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// updateAsset handles updating an existing asset
func (s *Server) updateAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	var req models.UpdateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	type set struct {
		sql string
		val interface{}
	}
	sets := make([]set, 0, 10)
	arg := 1

	if req.AssetType != nil {
		sets = append(sets, set{fmt.Sprintf("asset_type = $%d", arg), *req.AssetType})
		arg++
	}
	if req.Name != nil {
		sets = append(sets, set{fmt.Sprintf("name = $%d", arg), nullIfEmpty(req.Name)})
		arg++
	}
	if req.Vendor != nil {
		sets = append(sets, set{fmt.Sprintf("vendor = $%d", arg), nullIfEmpty(req.Vendor)})
		arg++
	}
	if req.Model != nil {
		sets = append(sets, set{fmt.Sprintf("model = $%d", arg), nullIfEmpty(req.Model)})
		arg++
	}
	if req.Serial != nil {
		sets = append(sets, set{fmt.Sprintf("serial = $%d", arg), nullIfEmpty(req.Serial)})
		arg++
	}
	if req.MgmtIP != nil {
		if ip := net.ParseIP(*req.MgmtIP); ip != nil {
			sets = append(sets, set{fmt.Sprintf("mgmt_ip = $%d", arg), ip.String()})
		} else {
			http.Error(w, "invalid mgmt_ip format", 400)
			return
		}
		arg++
	}
	if req.Status != nil {
		sets = append(sets, set{fmt.Sprintf("status = $%d", arg), nullIfEmpty(req.Status)})
		arg++
	}
	if req.Notes != nil {
		sets = append(sets, set{fmt.Sprintf("notes = $%d", arg), nullIfEmpty(req.Notes)})
		arg++
	}
	if req.Extras != nil {
		extrasJSON, err := json.Marshal(req.Extras)
		if err != nil {
			http.Error(w, "invalid extras JSON", 400)
			return
		}
		sets = append(sets, set{fmt.Sprintf("extras = $%d", arg), extrasJSON})
		arg++
	}

	if len(sets) == 0 {
		http.Error(w, "no fields to update", 400)
		return
	}

	args := make([]interface{}, 0, len(sets)+2)
	sqlStr := "UPDATE assets SET "
	for i, sset := range sets {
		if i > 0 {
			sqlStr += ", "
		}
		sqlStr += sset.sql
		args = append(args, sset.val)
	}
	sqlStr += fmt.Sprintf(" WHERE id = $%d AND org_id = $%d RETURNING id, org_id, site_id, asset_type, name, vendor, model, serial, mgmt_ip, status, notes, extras, created_at, updated_at", len(args)+1, len(args)+2)
	args = append(args, id, orgID)

	q := dbFrom(r.Context(), s.DB)
	var out models.Asset
	var mgmtIPStr *string
	var extrasJSON []byte
	if err := q.QueryRowContext(r.Context(), sqlStr, args...).Scan(&out.ID, &out.OrgID, &out.SiteID, &out.AssetType, &out.Name, &out.Vendor, &out.Model, &out.Serial, &mgmtIPStr, &out.Status, &out.Notes, &extrasJSON, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "asset with this serial already exists for this site and type", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}

	// Parse mgmt_ip
	if mgmtIPStr != nil {
		if ip := net.ParseIP(*mgmtIPStr); ip != nil {
			out.MgmtIP = &ip
		}
	}

	// Parse extras JSON
	if len(extrasJSON) > 0 {
		if err := json.Unmarshal(extrasJSON, &out.Extras); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	// Update subtype records if provided
	if req.Switch != nil {
		_, err := q.ExecContext(r.Context(), `
			INSERT INTO asset_switches (asset_id, ports_total, poe, uplink_info, firmware)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (asset_id) DO UPDATE SET
				ports_total = EXCLUDED.ports_total,
				poe = EXCLUDED.poe,
				uplink_info = EXCLUDED.uplink_info,
				firmware = EXCLUDED.firmware
		`, out.ID, req.Switch.PortsTotal, req.Switch.POE, req.Switch.UplinkInfo, req.Switch.Firmware)
		if err != nil {
			http.Error(w, "failed to update switch subtype: "+err.Error(), 500)
			return
		}
	}

	if req.VLAN != nil {
		_, err := q.ExecContext(r.Context(), `
			INSERT INTO asset_vlans (asset_id, vlan_id, subnet, gateway, purpose)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (asset_id) DO UPDATE SET
				vlan_id = EXCLUDED.vlan_id,
				subnet = EXCLUDED.subnet,
				gateway = EXCLUDED.gateway,
				purpose = EXCLUDED.purpose
		`, out.ID, req.VLAN.VLANID, req.VLAN.Subnet, req.VLAN.Gateway, req.VLAN.Purpose)
		if err != nil {
			http.Error(w, "failed to update VLAN subtype: "+err.Error(), 500)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// deleteAsset handles deleting an asset
func (s *Server) deleteAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	q := dbFrom(r.Context(), s.DB)
	res, err := q.ExecContext(r.Context(), `DELETE FROM assets WHERE id = $1 AND org_id = $2`, id, orgID)
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

// listSwitches handles listing switches with subtype data
func (s *Server) listSwitches(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)
	orgID := auth.OrgIDFromContext(r.Context())

	// Cap limit at 100
	if params.limit > 100 {
		params.limit = 100
	}

	clauses := []string{"a.asset_type = 'switch'"}
	args := []interface{}{}
	arg := 1

	// org filter
	clauses = append(clauses, fmt.Sprintf("a.org_id = $%d", arg))
	args = append(args, orgID)
	arg++

	// optional site filter
	if siteIDStr := strings.TrimSpace(r.URL.Query().Get("site_id")); siteIDStr != "" {
		if siteID, err := strconv.ParseInt(siteIDStr, 10, 64); err == nil {
			clauses = append(clauses, fmt.Sprintf("a.site_id = $%d", arg))
			args = append(args, siteID)
			arg++
		}
	}

	// optional text search on name
	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("a.name ILIKE $%d", arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	whereClause := " WHERE " + strings.Join(clauses, " AND ")

	// Build the main query with COUNT(*) OVER() to get total count
	sqlStr := fmt.Sprintf(`
		SELECT a.id, a.org_id, a.site_id, a.asset_type, a.name, a.vendor, a.model, a.serial, a.mgmt_ip, a.status, a.notes, a.extras, a.created_at, a.updated_at,
		       s.ports_total, s.poe, s.uplink_info, s.firmware,
		       COUNT(*) OVER() as total_count
		FROM assets a
		LEFT JOIN asset_switches s ON a.id = s.asset_id%s`, whereClause)

	allowedSort := map[string]string{
		"id":         "a.id",
		"name":       "a.name",
		"vendor":     "a.vendor",
		"created_at": "a.created_at",
		"updated_at": "a.updated_at",
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

	switches := []interface{}{}
	var totalCount int
	for rows.Next() {
		var asset models.Asset
		var switchData models.AssetSwitch
		var mgmtIPStr *string
		var extrasJSON []byte
		if err := rows.Scan(&asset.ID, &asset.OrgID, &asset.SiteID, &asset.AssetType, &asset.Name, &asset.Vendor, &asset.Model, &asset.Serial, &mgmtIPStr, &asset.Status, &asset.Notes, &extrasJSON, &asset.CreatedAt, &asset.UpdatedAt, &switchData.PortsTotal, &switchData.POE, &switchData.UplinkInfo, &switchData.Firmware, &totalCount); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Parse mgmt_ip
		if mgmtIPStr != nil {
			if ip := net.ParseIP(*mgmtIPStr); ip != nil {
				asset.MgmtIP = &ip
			}
		}

		// Parse extras JSON
		if len(extrasJSON) > 0 {
			if err := json.Unmarshal(extrasJSON, &asset.Extras); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}

		// Create asset with switch data
		assetWithSwitch := models.AssetWithSubtypes{
			Asset:  asset,
			Switch: &switchData,
		}

		switches = append(switches, assetWithSwitch)
	}

	sendListResponse(w, switches, totalCount, params)
}

// listVLANs handles listing VLANs with subtype data
func (s *Server) listVLANs(w http.ResponseWriter, r *http.Request) {
	params := parseListParams(r)
	orgID := auth.OrgIDFromContext(r.Context())

	// Cap limit at 100
	if params.limit > 100 {
		params.limit = 100
	}

	clauses := []string{"a.asset_type = 'vlan'"}
	args := []interface{}{}
	arg := 1

	// org filter
	clauses = append(clauses, fmt.Sprintf("a.org_id = $%d", arg))
	args = append(args, orgID)
	arg++

	// optional site filter
	if siteIDStr := strings.TrimSpace(r.URL.Query().Get("site_id")); siteIDStr != "" {
		if siteID, err := strconv.ParseInt(siteIDStr, 10, 64); err == nil {
			clauses = append(clauses, fmt.Sprintf("a.site_id = $%d", arg))
			args = append(args, siteID)
			arg++
		}
	}

	// optional text search on name
	if params.q != "" {
		clauses = append(clauses, fmt.Sprintf("a.name ILIKE $%d", arg))
		args = append(args, "%"+params.q+"%")
		arg++
	}

	whereClause := " WHERE " + strings.Join(clauses, " AND ")

	// Build the main query with COUNT(*) OVER() to get total count
	sqlStr := fmt.Sprintf(`
		SELECT a.id, a.org_id, a.site_id, a.asset_type, a.name, a.vendor, a.model, a.serial, a.mgmt_ip, a.status, a.notes, a.extras, a.created_at, a.updated_at,
		       v.vlan_id, v.subnet, v.gateway, v.purpose,
		       COUNT(*) OVER() as total_count
		FROM assets a
		LEFT JOIN asset_vlans v ON a.id = v.asset_id%s`, whereClause)

	allowedSort := map[string]string{
		"id":         "a.id",
		"name":       "a.name",
		"vlan_id":    "v.vlan_id",
		"created_at": "a.created_at",
		"updated_at": "a.updated_at",
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

	vlans := []interface{}{}
	var totalCount int
	for rows.Next() {
		var asset models.Asset
		var vlanData models.AssetVLAN
		var mgmtIPStr *string
		var extrasJSON []byte
		var gatewayStr *string
		if err := rows.Scan(&asset.ID, &asset.OrgID, &asset.SiteID, &asset.AssetType, &asset.Name, &asset.Vendor, &asset.Model, &asset.Serial, &mgmtIPStr, &asset.Status, &asset.Notes, &extrasJSON, &asset.CreatedAt, &asset.UpdatedAt, &vlanData.VLANID, &vlanData.Subnet, &gatewayStr, &vlanData.Purpose, &totalCount); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Parse mgmt_ip
		if mgmtIPStr != nil {
			if ip := net.ParseIP(*mgmtIPStr); ip != nil {
				asset.MgmtIP = &ip
			}
		}

		// Parse gateway
		if gatewayStr != nil {
			if ip := net.ParseIP(*gatewayStr); ip != nil {
				vlanData.Gateway = &ip
			}
		}

		// Parse extras JSON
		if len(extrasJSON) > 0 {
			if err := json.Unmarshal(extrasJSON, &asset.Extras); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}

		// Create asset with VLAN data
		assetWithVLAN := models.AssetWithSubtypes{
			Asset: asset,
			VLAN:  &vlanData,
		}

		vlans = append(vlans, assetWithVLAN)
	}

	sendListResponse(w, vlans, totalCount, params)
}

// getSiteAssetCategories handles getting dynamic site asset categories
func (s *Server) getSiteAssetCategories(w http.ResponseWriter, r *http.Request) {
	siteID := chi.URLParam(r, "id")
	orgID := auth.OrgIDFromContext(r.Context())

	// Validate site_id
	if _, err := strconv.ParseInt(siteID, 10, 64); err != nil {
		http.Error(w, "invalid site_id", 400)
		return
	}

	q := dbFrom(r.Context(), s.DB)
	rows, err := q.QueryContext(r.Context(), `
		SELECT org_id, site_id, asset_type, asset_count
		FROM site_asset_categories
		WHERE org_id = $1 AND site_id = $2
		ORDER BY asset_type
	`, orgID, siteID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	categories := []models.SiteAssetCategory{}
	for rows.Next() {
		var cat models.SiteAssetCategory
		if err := rows.Scan(&cat.OrgID, &cat.SiteID, &cat.AssetType, &cat.AssetCount); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		categories = append(categories, cat)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(categories); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
