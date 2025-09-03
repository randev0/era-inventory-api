package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tealeg/xlsx/v3"
	"gopkg.in/yaml.v3"
)

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

type ImportStats struct {
	SheetName string
	RowsRead  int
	Inserted  int
	Updated   int
	Skipped   int
	Errors    []string
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: import_excel --file=path.xlsx --org-id=... --site-id=... --mapping=configs/mapping/mbip_equipment.yaml")
		os.Exit(1)
	}

	var filePath, orgIDStr, siteIDStr, mappingPath string

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--file=") {
			filePath = strings.TrimPrefix(arg, "--file=")
		} else if strings.HasPrefix(arg, "--org-id=") {
			orgIDStr = strings.TrimPrefix(arg, "--org-id=")
		} else if strings.HasPrefix(arg, "--site-id=") {
			siteIDStr = strings.TrimPrefix(arg, "--site-id=")
		} else if strings.HasPrefix(arg, "--mapping=") {
			mappingPath = strings.TrimPrefix(arg, "--mapping=")
		}
	}

	if filePath == "" || orgIDStr == "" || siteIDStr == "" || mappingPath == "" {
		fmt.Println("Error: All parameters are required")
		fmt.Println("Usage: import_excel --file=path.xlsx --org-id=... --site-id=... --mapping=configs/mapping/mbip_equipment.yaml")
		os.Exit(1)
	}

	orgID, err := strconv.ParseInt(orgIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid org-id: %v", err)
	}

	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid site-id: %v", err)
	}

	// Load mapping configuration
	mapping, err := loadMappingConfig(mappingPath)
	if err != nil {
		log.Fatalf("Failed to load mapping config: %v", err)
	}

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/era?sslmode=disable"
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Set org context for RLS
	_, err = db.Exec("SET LOCAL app.current_org_id = $1", orgID)
	if err != nil {
		log.Fatalf("Failed to set org context: %v", err)
	}

	// Open Excel file
	xlFile, err := xlsx.OpenFile(filePath)
	if err != nil {
		log.Fatalf("Failed to open Excel file: %v", err)
	}

	fmt.Printf("Importing from %s to org_id=%d, site_id=%d\n", filePath, orgID, siteID)
	fmt.Println("=" + strings.Repeat("=", 60))

	var allStats []ImportStats

	// Process each sheet
	for _, sheet := range xlFile.Sheets {
		sheetName := sheet.Name
		sheetConfig, exists := mapping.Sheets[sheetName]
		if !exists {
			fmt.Printf("Skipping sheet '%s' (no mapping found)\n", sheetName)
			continue
		}

		fmt.Printf("\nProcessing sheet: %s\n", sheetName)
		stats := processSheet(db, sheet, sheetConfig, orgID, siteID, mapping.DefaultOrgFields)
		allStats = append(allStats, stats)

		fmt.Printf("  Rows read: %d\n", stats.RowsRead)
		fmt.Printf("  Inserted: %d\n", stats.Inserted)
		fmt.Printf("  Updated: %d\n", stats.Updated)
		fmt.Printf("  Skipped: %d\n", stats.Skipped)
		if len(stats.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(stats.Errors))
			for _, err := range stats.Errors {
				fmt.Printf("    - %s\n", err)
			}
		}
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("IMPORT SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	totalRead, totalInserted, totalUpdated, totalSkipped := 0, 0, 0, 0
	for _, stats := range allStats {
		totalRead += stats.RowsRead
		totalInserted += stats.Inserted
		totalUpdated += stats.Updated
		totalSkipped += stats.Skipped
	}

	fmt.Printf("Total rows processed: %d\n", totalRead)
	fmt.Printf("Total inserted: %d\n", totalInserted)
	fmt.Printf("Total updated: %d\n", totalUpdated)
	fmt.Printf("Total skipped: %d\n", totalSkipped)
}

func loadMappingConfig(path string) (*MappingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config MappingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func processSheet(db *sql.DB, sheet *xlsx.Sheet, config SheetConfig, orgID, siteID int64, defaultFields map[string]interface{}) ImportStats {
	stats := ImportStats{SheetName: sheet.Name}

	// Get header row (first row)
	headerRow, err := sheet.Row(0)
	if err != nil {
		stats.Errors = append(stats.Errors, "Failed to read header row: "+err.Error())
		return stats
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

		stats.RowsRead++

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
			stats.Skipped++
			rowIdx++
			continue
		}

		// Build asset data
		assetData, err := buildAssetData(rowData, config, defaultFields, aliasMap)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowIdx+1, err))
			stats.Skipped++
			rowIdx++
			continue
		}

		// Check if asset already exists
		existingID, err := findExistingAsset(db, assetData, config.NaturalKey, orgID, siteID)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowIdx+1, err))
			stats.Skipped++
			rowIdx++
			continue
		}

		if existingID > 0 {
			// Update existing asset
			if err := updateAsset(db, existingID, assetData, config); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowIdx+1, err))
				stats.Skipped++
				rowIdx++
				continue
			}
			stats.Updated++
		} else {
			// Insert new asset
			if err := insertAsset(db, assetData, config, orgID, siteID); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("Row %d: %v", rowIdx+1, err))
				stats.Skipped++
				rowIdx++
				continue
			}
			stats.Inserted++
		}

		rowIdx++
	}

	return stats
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
	case "string":
		return value, nil
	case "int":
		return strconv.Atoi(value)
	case "bool":
		value = strings.ToLower(value)
		return value == "yes" || value == "y" || value == "true" || value == "1", nil
	case "ip":
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", value)
		}
		return ip, nil
	case "timestamp":
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

func findExistingAsset(db *sql.DB, assetData map[string]interface{}, naturalKey []string, orgID, siteID int64) (int64, error) {
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
				err := db.QueryRow(query, args...).Scan(&id)
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

func insertAsset(db *sql.DB, assetData map[string]interface{}, config SheetConfig, orgID, siteID int64) error {
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
	err := db.QueryRow(query, assetValues...).Scan(&assetID)
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

			_, err = db.Exec(subtypeQuery, subtypeValues...)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func updateAsset(db *sql.DB, assetID int64, assetData map[string]interface{}, config SheetConfig) error {
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

		_, err := db.Exec(query, values...)
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
			_, err := db.Exec(subtypeQuery, allValues...)
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
