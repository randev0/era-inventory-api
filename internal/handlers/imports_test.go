package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"era-inventory-api/internal/auth"
	"era-inventory-api/pkg/importer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportsHandler_UploadExcel(t *testing.T) {
	// Create a mock handler (without real database for unit tests)
	handler := &ImportsHandler{
		DB:         nil, // Will be nil for unit tests
		MaxBytes:   20 << 20,
		DefaultMap: "configs/mapping/mbip_equipment.yaml",
	}

	t.Run("Rejects non-multipart content type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/imports/excel", nil)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		handler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "content-type must be multipart/form-data")
	})

	t.Run("Rejects missing site_id", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		handler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "site_id is required")
	})

	t.Run("Rejects invalid site_id", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("site_id", "invalid")
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		handler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "site_id is required")
	})

	t.Run("Rejects missing file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("site_id", "5")
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		handler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "file is required")
	})

	t.Run("Rejects non-xlsx file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("site_id", "5")

		// Create a fake file with .xls extension
		fileWriter, _ := writer.CreateFormFile("file", "test.xls")
		fileWriter.Write([]byte("fake excel content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		handler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "only .xlsx files are accepted")
	})

	t.Run("Accepts valid xlsx file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("site_id", "5")
		writer.WriteField("dry_run", "true")

		// Create a fake file with .xlsx extension
		fileWriter, _ := writer.CreateFormFile("file", "test.xlsx")
		fileWriter.Write([]byte("fake excel content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		handler.UploadExcel(w, req)

		// Should fail due to nil database, but not due to validation
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
	})
}

func TestIsXLSX(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Valid xlsx", "test.xlsx", true},
		{"Valid xlsx uppercase", "TEST.XLSX", true},
		{"Valid xlsx mixed case", "Test.XlSx", true},
		{"Invalid xls", "test.xls", false},
		{"Invalid xlsm", "test.xlsm", false},
		{"Invalid txt", "test.txt", false},
		{"No extension", "test", false},
		{"Empty filename", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &multipart.FileHeader{
				Filename: tt.filename,
			}
			result := isXLSX(header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteJSON(t *testing.T) {
	t.Run("Writes JSON response", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]interface{}{
			"message": "test",
			"count":   42,
		}

		writeJSON(w, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "test", response["message"])
		assert.Equal(t, float64(42), response["count"])
	})
}

// Mock test for the importer library
func TestImporterLibrary(t *testing.T) {
	t.Run("ImportOptions validation", func(t *testing.T) {
		opts := importer.ImportOptions{
			OrgID:       1,
			SiteID:      5,
			MappingPath: "test.yaml",
			DryRun:      true,
			MaxErrors:   50,
		}

		assert.Equal(t, int64(1), opts.OrgID)
		assert.Equal(t, int64(5), opts.SiteID)
		assert.Equal(t, "test.yaml", opts.MappingPath)
		assert.True(t, opts.DryRun)
		assert.Equal(t, 50, opts.MaxErrors)
	})

	t.Run("RowError structure", func(t *testing.T) {
		error := importer.RowError{
			Sheet:   "Equipment",
			Row:     5,
			Message: "Invalid IP address",
		}

		assert.Equal(t, "Equipment", error.Sheet)
		assert.Equal(t, 5, error.Row)
		assert.Equal(t, "Invalid IP address", error.Message)
	})

	t.Run("SheetSummary structure", func(t *testing.T) {
		summary := importer.SheetSummary{
			Name:     "Equipment",
			Inserted: 10,
			Updated:  5,
			Skipped:  2,
			Errors:   1,
			Samples: []importer.RowError{
				{Sheet: "Equipment", Row: 5, Message: "Test error"},
			},
		}

		assert.Equal(t, "Equipment", summary.Name)
		assert.Equal(t, 10, summary.Inserted)
		assert.Equal(t, 5, summary.Updated)
		assert.Equal(t, 2, summary.Skipped)
		assert.Equal(t, 1, summary.Errors)
		assert.Len(t, summary.Samples, 1)
	})

	t.Run("ImportSummary structure", func(t *testing.T) {
		summary := importer.ImportSummary{
			Inserted: 15,
			Updated:  8,
			Skipped:  3,
			Errors:   2,
			Sheets: []importer.SheetSummary{
				{Name: "Equipment", Inserted: 15, Updated: 8, Skipped: 3, Errors: 2},
			},
			DryRun: false,
		}

		assert.Equal(t, 15, summary.Inserted)
		assert.Equal(t, 8, summary.Updated)
		assert.Equal(t, 3, summary.Skipped)
		assert.Equal(t, 2, summary.Errors)
		assert.Len(t, summary.Sheets, 1)
		assert.False(t, summary.DryRun)
	})
}
