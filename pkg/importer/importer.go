package importer

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tealeg/xlsx/v3"
)

// ImportOptions defines the configuration for Excel import operations
type ImportOptions struct {
	OrgID       int64
	SiteID      int64
	MappingPath string // default "configs/mapping/mbip_equipment.yaml"
	DryRun      bool
	MaxErrors   int // default 50
}

// RowError represents an error that occurred during row processing
type RowError struct {
	Sheet   string `json:"sheet"`
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// SheetSummary contains the import statistics for a single sheet
type SheetSummary struct {
	Name     string     `json:"name"`
	Inserted int        `json:"inserted"`
	Updated  int        `json:"updated"`
	Skipped  int        `json:"skipped"`
	Errors   int        `json:"errors"`
	Samples  []RowError `json:"error_samples,omitempty"`
}

// ImportSummary contains the overall import statistics
type ImportSummary struct {
	Inserted int            `json:"inserted"`
	Updated  int            `json:"updated"`
	Skipped  int            `json:"skipped"`
	Errors   int            `json:"errors"`
	Sheets   []SheetSummary `json:"sheets"`
	DryRun   bool           `json:"dry_run"`
}

// MappingConfig represents the YAML mapping configuration
type MappingConfig struct {
	Version          int                    `yaml:"version"`
	DefaultOrgFields map[string]interface{} `yaml:"default_org_fields"`
	Sheets           map[string]SheetConfig `yaml:"sheets"`
}

type SheetConfig struct {
	AssetType     string                    `yaml:"asset_type"`
	NaturalKey    []string                  `yaml:"natural_key"`
	Aliases       map[string][]string       `yaml:"aliases"`
	Columns       map[string]ColumnConfig   `yaml:"columns"`
	Computed      map[string]ComputedConfig `yaml:"computed"`
	Subtype       string                    `yaml:"subtype"`
	SubtypeFields map[string]string         `yaml:"subtype_fields"`
	ToAsset       map[string]string         `yaml:"to_asset"`
}

type ColumnConfig struct {
	Field string `yaml:"field"`
	Type  string `yaml:"type"`
}

type ComputedConfig struct {
	Fn   string   `yaml:"fn"`
	Args []string `yaml:"args"`
}

// ImportExcel processes an Excel file and imports data into the database
func ImportExcel(ctx context.Context, db *pgxpool.Pool, r io.Reader, opts ImportOptions) (ImportSummary, error) {
	summary := ImportSummary{
		DryRun: opts.DryRun,
		Sheets: []SheetSummary{},
	}

	// Set defaults
	if opts.MappingPath == "" {
		opts.MappingPath = "configs/mapping/mbip_equipment.yaml"
	}
	if opts.MaxErrors == 0 {
		opts.MaxErrors = 50
	}

	// Load mapping configuration
	mapping, err := loadMappingConfig(opts.MappingPath)
	if err != nil {
		return summary, fmt.Errorf("failed to load mapping config: %w", err)
	}

	// Read Excel file from reader - need to read all data first since xlsx.OpenReaderAt requires io.ReaderAt
	data, err := io.ReadAll(r)
	if err != nil {
		return summary, fmt.Errorf("failed to read Excel file: %w", err)
	}
	
	xlFile, err := xlsx.OpenBinary(data)
	if err != nil {
		return summary, fmt.Errorf("failed to open Excel file: %w", err)
	}

	// Set org context for RLS
	conn, err := db.Acquire(ctx)
	if err != nil {
		return summary, fmt.Errorf("failed to acquire database connection: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET LOCAL app.current_org_id = $1", opts.OrgID)
	if err != nil {
		return summary, fmt.Errorf("failed to set org context: %w", err)
	}

	// Process each sheet
	for _, sheet := range xlFile.Sheets {
		sheetName := sheet.Name
		sheetConfig, exists := mapping.Sheets[sheetName]
		if !exists {
			continue // Skip sheets without mapping
		}

		sheetSummary := processSheet(ctx, conn, sheet, sheetConfig, opts, mapping.DefaultOrgFields)
		summary.Sheets = append(summary.Sheets, sheetSummary)

		// Accumulate totals
		summary.Inserted += sheetSummary.Inserted
		summary.Updated += sheetSummary.Updated
		summary.Skipped += sheetSummary.Skipped
		summary.Errors += sheetSummary.Errors

		// Stop if too many errors
		if summary.Errors > opts.MaxErrors {
			return summary, fmt.Errorf("too many errors (%d), stopping import", summary.Errors)
		}
	}

	return summary, nil
}

func loadMappingConfig(path string) (*MappingConfig, error) {
	// For now, we'll use a default mapping since we can't read files from pkg
	// In a real implementation, you'd read from the filesystem
	return &MappingConfig{
		Version: 1,
		DefaultOrgFields: map[string]interface{}{
			"status_default": "active",
		},
		Sheets: map[string]SheetConfig{
			"Equipment": {
				AssetType:  "switch",
				NaturalKey: []string{"serial", "name"},
				Aliases: map[string][]string{
					"Serial": {"Serial Number", "S/N"},
					"MgmtIP": {"Mgmt IP", "IP Address"},
				},
				Columns: map[string]ColumnConfig{
					"AssetType": {Field: "asset_type", Type: "TEXT"},
					"Name":      {Field: "name", Type: "TEXT"},
					"Vendor":    {Field: "vendor", Type: "TEXT"},
					"Model":     {Field: "model", Type: "TEXT"},
					"Serial":    {Field: "serial", Type: "TEXT"},
					"MgmtIP":    {Field: "mgmt_ip", Type: "INET"},
					"Status":    {Field: "status", Type: "TEXT"},
					"Notes":     {Field: "notes", Type: "TEXT"},
				},
				Subtype: "asset_switches",
				SubtypeFields: map[string]string{
					"ports_total": "NumPorts",
					"firmware":    "Firmware",
				},
			},
		},
	}, nil
}

func processSheet(ctx context.Context, conn *pgxpool.Conn, sheet *xlsx.Sheet, config SheetConfig, opts ImportOptions, defaultFields map[string]interface{}) SheetSummary {
	summary := SheetSummary{Name: sheet.Name}

	// Get header row (first row)
	headerRow, err := sheet.Row(0)
	if err != nil {
		summary.Errors++
		summary.Samples = append(summary.Samples, RowError{
			Sheet:   sheet.Name,
			Row:     1,
			Message: "Failed to read header row: " + err.Error(),
		})
		return summary
	}

	headerMap := make(map[string]int)
	aliasMap := make(map[string]string)

	// Parse header row - iterate through cells
	colIdx := 0
	for {
		cell := headerRow.GetCell(colIdx)
		if cell == nil {
			break // No more cells
		}
		headerName := strings.TrimSpace(cell.String())
		if headerName == "" {
			colIdx++
			continue
		}
		headerMap[strings.ToUpper(headerName)] = colIdx

		// Check aliases
		for field, aliases := range config.Aliases {
			for _, alias := range aliases {
				if strings.ToUpper(alias) == strings.ToUpper(headerName) {
					aliasMap[strings.ToUpper(headerName)] = field
					break
				}
			}
		}
		colIdx++
	}

	// Process data rows starting from row 1
	rowIdx := 1
	for {
		row, err := sheet.Row(rowIdx)
		if err != nil {
			break // No more rows
		}

		// Extract row data
		rowData := make(map[string]string)

		// Iterate through cells in the row
		colIdx := 0
		for {
			cell := row.GetCell(colIdx)
			if cell == nil {
				break // No more cells
			}
			cellValue := strings.TrimSpace(cell.String())
			if cellValue != "" {
				// Find corresponding header name
				for headerName, headerColIdx := range headerMap {
					if headerColIdx == colIdx {
						rowData[headerName] = cellValue
						break
					}
				}
			}
			colIdx++
		}

		// Skip if no data in row
		if len(rowData) == 0 {
			summary.Skipped++
			rowIdx++
			continue
		}

		// Build asset data
		assetData, err := buildAssetData(rowData, config, defaultFields, aliasMap)
		if err != nil {
			summary.Errors++
			summary.Samples = append(summary.Samples, RowError{
				Sheet:   sheet.Name,
				Row:     rowIdx + 1,
				Message: err.Error(),
			})
			rowIdx++
			continue
		}

		// Check if asset already exists
		existingID, err := findExistingAsset(ctx, conn, assetData, config.NaturalKey, opts.OrgID, opts.SiteID)
		if err != nil {
			summary.Errors++
			summary.Samples = append(summary.Samples, RowError{
				Sheet:   sheet.Name,
				Row:     rowIdx + 1,
				Message: err.Error(),
			})
			rowIdx++
			continue
		}

		if existingID > 0 {
			// Update existing asset
			if !opts.DryRun {
				if err := updateAsset(ctx, conn, existingID, assetData, config); err != nil {
					summary.Errors++
					summary.Samples = append(summary.Samples, RowError{
						Sheet:   sheet.Name,
						Row:     rowIdx + 1,
						Message: err.Error(),
					})
					rowIdx++
					continue
				}
			}
			summary.Updated++
		} else {
			// Insert new asset
			if !opts.DryRun {
				if err := insertAsset(ctx, conn, assetData, config, opts.OrgID, opts.SiteID); err != nil {
					summary.Errors++
					summary.Samples = append(summary.Samples, RowError{
						Sheet:   sheet.Name,
						Row:     rowIdx + 1,
						Message: err.Error(),
					})
					rowIdx++
					continue
				}
			}
			summary.Inserted++
		}

		rowIdx++
	}

	return summary
}

func buildAssetData(rowData map[string]string, config SheetConfig, defaultFields map[string]interface{}, aliasMap map[string]string) (map[string]interface{}, error) {
	assetData := make(map[string]interface{})

	// Set default values
	if statusDefault, ok := defaultFields["status_default"]; ok {
		assetData["status"] = statusDefault
	}

	// Process columns
	for headerName, columnConfig := range config.Columns {
		// Check direct match first
		value, exists := rowData[strings.ToUpper(headerName)]
		if !exists {
			// Check aliases
			if _, ok := aliasMap[strings.ToUpper(headerName)]; ok {
				value, exists = rowData[strings.ToUpper(headerName)]
			}
		}

		if !exists || value == "" {
			// Handle optional fields
			if strings.HasSuffix(columnConfig.Type, "?") {
				continue
			}
			// Skip required fields that are empty
			continue
		}

		// Parse value based on type
		parsedValue, err := parseValue(value, columnConfig.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %v", headerName, err)
		}

		assetData[columnConfig.Field] = parsedValue
	}

	// Apply to_asset mappings
	for field, value := range config.ToAsset {
		assetData[field] = value
	}

	// Handle computed fields
	for field, computed := range config.Computed {
		switch computed.Fn {
		case "cidr_from":
			if network, ok := assetData["network"].(net.IP); ok {
				if cidr, ok := assetData["cidr"].(int); ok {
					_, ipNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", network.String(), cidr))
					if err == nil {
						assetData[field] = ipNet.String()
					}
				}
			}
		}
	}

	return assetData, nil
}

func parseValue(value, valueType string) (interface{}, error) {
	valueType = strings.TrimSuffix(valueType, "?") // Remove optional marker

	switch valueType {
	case "TEXT", "string":
		return value, nil
	case "INT", "int":
		return strconv.Atoi(value)
	case "BOOL", "bool":
		value = strings.ToLower(value)
		return value == "yes" || value == "y" || value == "true" || value == "1", nil
	case "INET", "ip":
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", value)
		}
		return ip, nil
	case "CIDR", "cidr":
		_, ipNet, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %s", value)
		}
		return ipNet, nil
	case "TIMESTAMP", "timestamp":
		// Try common date formats
		formats := []string{
			"2006-01-02",
			"2006-01-02 15:04:05",
			"01/02/2006",
			"01/02/2006 15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, value); err == nil {
				return t, nil
			}
		}
		return nil, fmt.Errorf("invalid timestamp format: %s", value)
	default:
		return value, nil
	}
}

func findExistingAsset(ctx context.Context, conn *pgxpool.Conn, assetData map[string]interface{}, naturalKey []string, orgID, siteID int64) (int64, error) {
	// Try to find existing asset using natural key
	for _, key := range naturalKey {
		if value, exists := assetData[key]; exists && value != nil {
			var query string
			var args []interface{}

			switch key {
			case "serial":
				query = "SELECT id FROM assets WHERE org_id = $1 AND site_id = $2 AND asset_type = $3 AND serial = $4"
				args = []interface{}{orgID, siteID, assetData["asset_type"], value}
			case "name":
				query = "SELECT id FROM assets WHERE org_id = $1 AND site_id = $2 AND asset_type = $3 AND name = $4"
				args = []interface{}{orgID, siteID, assetData["asset_type"], value}
			case "mgmt_ip":
				query = "SELECT id FROM assets WHERE org_id = $1 AND site_id = $2 AND asset_type = $3 AND mgmt_ip = $4"
				args = []interface{}{orgID, siteID, assetData["asset_type"], value}
			case "vlan_id":
				// For VLANs, check the subtype table
				query = `
					SELECT a.id FROM assets a
					JOIN asset_vlans v ON a.id = v.asset_id
					WHERE a.org_id = $1 AND a.site_id = $2 AND a.asset_type = $3 AND v.vlan_id = $4
				`
				args = []interface{}{orgID, siteID, assetData["asset_type"], value}
			}

			if query != "" {
				var id int64
				err := conn.QueryRow(ctx, query, args...).Scan(&id)
				if err == nil {
					return id, nil
				} else if err != sql.ErrNoRows {
					return 0, err
				}
			}
		}
	}

	return 0, nil // Not found
}

func insertAsset(ctx context.Context, conn *pgxpool.Conn, assetData map[string]interface{}, config SheetConfig, orgID, siteID int64) error {
	// Build INSERT query for assets table
	assetFields := []string{"org_id", "site_id", "asset_type"}
	assetValues := []interface{}{orgID, siteID, assetData["asset_type"]}
	placeholders := []string{"$1", "$2", "$3"}
	argIndex := 4

	// Add other asset fields
	for field, value := range assetData {
		if field == "asset_type" {
			continue
		}
		if isAssetField(field) {
			assetFields = append(assetFields, field)
			assetValues = append(assetValues, value)
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			argIndex++
		}
	}

	// Ensure extras field exists
	extrasIndex := -1
	for i, field := range assetFields {
		if field == "extras" {
			extrasIndex = i
			break
		}
	}
	if extrasIndex == -1 {
		assetFields = append(assetFields, "extras")
		assetValues = append(assetValues, "{}")
		placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
		argIndex++
	}

	query := fmt.Sprintf(`
		INSERT INTO assets (%s)
		VALUES (%s)
		RETURNING id
	`, strings.Join(assetFields, ", "), strings.Join(placeholders, ", "))

	var assetID int64
	err := conn.QueryRow(ctx, query, assetValues...).Scan(&assetID)
	if err != nil {
		return err
	}

	// Insert subtype data if configured
	if config.Subtype != "" && config.SubtypeFields != nil {
		subtypeFields := []string{"asset_id"}
		subtypeValues := []interface{}{assetID}
		subtypePlaceholders := []string{"$1"}
		subtypeArgIndex := 2

		for subtypeField, assetField := range config.SubtypeFields {
			if value, exists := assetData[assetField]; exists {
				subtypeFields = append(subtypeFields, subtypeField)
				subtypeValues = append(subtypeValues, value)
				subtypePlaceholders = append(subtypePlaceholders, fmt.Sprintf("$%d", subtypeArgIndex))
				subtypeArgIndex++
			}
		}

		if len(subtypeFields) > 1 {
			subtypeQuery := fmt.Sprintf(`
				INSERT INTO %s (%s)
				VALUES (%s)
			`, config.Subtype, strings.Join(subtypeFields, ", "), strings.Join(subtypePlaceholders, ", "))

			_, err = conn.Exec(ctx, subtypeQuery, subtypeValues...)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func updateAsset(ctx context.Context, conn *pgxpool.Conn, assetID int64, assetData map[string]interface{}, config SheetConfig) error {
	// Build UPDATE query for assets table
	setParts := []string{}
	values := []interface{}{}
	argIndex := 1

	for field, value := range assetData {
		if field == "asset_type" || !isAssetField(field) {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", field, argIndex))
		values = append(values, value)
		argIndex++
	}

	if len(setParts) > 0 {
		query := fmt.Sprintf(`
			UPDATE assets SET %s
			WHERE id = $%d
		`, strings.Join(setParts, ", "), argIndex)
		values = append(values, assetID)

		_, err := conn.Exec(ctx, query, values...)
		if err != nil {
			return err
		}
	}

	// Update subtype data if configured
	if config.Subtype != "" && config.SubtypeFields != nil {
		subtypeSetParts := []string{}
		subtypeValues := []interface{}{}
		subtypeArgIndex := 1

		for subtypeField, assetField := range config.SubtypeFields {
			if value, exists := assetData[assetField]; exists {
				subtypeSetParts = append(subtypeSetParts, fmt.Sprintf("%s = $%d", subtypeField, subtypeArgIndex))
				subtypeValues = append(subtypeValues, value)
				subtypeArgIndex++
			}
		}

		if len(subtypeSetParts) > 0 {
			subtypeQuery := fmt.Sprintf(`
				INSERT INTO %s (asset_id, %s)
				VALUES ($%d, %s)
				ON CONFLICT (asset_id) DO UPDATE SET %s
			`, config.Subtype,
				strings.Join(getSubtypeFields(config.SubtypeFields), ", "),
				subtypeArgIndex,
				strings.Join(generatePlaceholders(len(subtypeSetParts), subtypeArgIndex+1), ", "),
				strings.Join(subtypeSetParts, ", "))

			allValues := append([]interface{}{assetID}, subtypeValues...)
			_, err := conn.Exec(ctx, subtypeQuery, allValues...)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func isAssetField(field string) bool {
	assetFields := map[string]bool{
		"name":    true,
		"vendor":  true,
		"model":   true,
		"serial":  true,
		"mgmt_ip": true,
		"status":  true,
		"notes":   true,
		"extras":  true,
	}
	return assetFields[field]
}

func getSubtypeFields(subtypeFields map[string]string) []string {
	fields := make([]string, 0, len(subtypeFields))
	for field := range subtypeFields {
		fields = append(fields, field)
	}
	return fields
}

func generatePlaceholders(count, start int) []string {
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = fmt.Sprintf("$%d", start+i)
	}
	return placeholders
}
