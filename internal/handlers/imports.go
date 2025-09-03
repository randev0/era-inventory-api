package handlers

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"era-inventory-api/internal/auth"
	"era-inventory-api/pkg/importer"
)

// ImportsHandler handles Excel import operations
type ImportsHandler struct {
	DB         *pgxpool.Pool
	MaxBytes   int64
	DefaultMap string
}

// NewImportsHandler creates a new imports handler
func NewImportsHandler(db *pgxpool.Pool) *ImportsHandler {
	return &ImportsHandler{
		DB:         db,
		MaxBytes:   20 << 20, // 20 MB
		DefaultMap: "configs/mapping/mbip_equipment.yaml",
	}
}

// UploadExcel handles Excel file uploads for asset import
func (h *ImportsHandler) UploadExcel(w http.ResponseWriter, r *http.Request) {
	// Limit body size
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxBytes)

	// Require multipart
	if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		http.Error(w, "content-type must be multipart/form-data", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(h.MaxBytes); err != nil {
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// SiteID required
	siteIDStr := r.FormValue("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil || siteID <= 0 {
		http.Error(w, "site_id is required and must be a positive integer", http.StatusBadRequest)
		return
	}

	dryRun := r.FormValue("dry_run") == "true"
	mapping := r.FormValue("mapping")
	if mapping == "" {
		mapping = h.DefaultMap
	}
	maxErrors := 50
	if v := r.FormValue("max_errors"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxErrors = n
		}
	}

	// File
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !isXLSX(header) {
		http.Error(w, "only .xlsx files are accepted", http.StatusBadRequest)
		return
	}

	// Auth / org context
	claims := auth.ClaimsFromContext(r.Context())
	orgID := claims.OrgID

	// Import (dry-run uses tx that rolls back inside importer or here)
	sum, impErr := importer.ImportExcel(r.Context(), h.DB, file, importer.ImportOptions{
		OrgID:       orgID,
		SiteID:      siteID,
		MappingPath: mapping,
		DryRun:      dryRun,
		MaxErrors:   maxErrors,
	})
	if impErr != nil {
		// Return a structured error payload consistent with your API
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":   "IMPORT_FAILED",
			"details": impErr.Error(),
			"data":    sum, // might include partial
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": sum,
		"meta": map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	})
}

// isXLSX checks if the uploaded file is an Excel .xlsx file
func isXLSX(h *multipart.FileHeader) bool {
	name := strings.ToLower(h.Filename)
	if !strings.HasSuffix(name, ".xlsx") {
		return false
	}
	// You may also sniff magic header if desired
	return true
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
