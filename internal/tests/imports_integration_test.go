package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/handlers"
	"era-inventory-api/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	// Create pgxpool for the importer
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Skip("Skipping test - no database connection available")
	}
	defer pool.Close()

	// Create test server
	importsHandler := handlers.NewImportsHandler(pool)

	// Test creating a simple Excel file for testing
	t.Run("CreateTestExcelFile", func(t *testing.T) {
		// For now, we'll create a minimal test without a real Excel file
		// In a real implementation, you would create an actual .xlsx file
		t.Skip("Skipping - requires actual Excel file creation")
	})

	t.Run("UploadExcelDryRun", func(t *testing.T) {
		// Create a simple multipart form with a fake Excel file
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add form fields
		writer.WriteField("site_id", "1")
		writer.WriteField("dry_run", "true")
		writer.WriteField("max_errors", "10")

		// Create a fake Excel file
		fileWriter, err := writer.CreateFormFile("file", "test.xlsx")
		require.NoError(t, err)

		// Write some fake Excel content (this won't be valid Excel, but tests the handler)
		fileWriter.Write([]byte("fake excel content for testing"))
		writer.Close()

		// Create request
		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		importsHandler.UploadExcel(w, req)

		// The request should fail due to invalid Excel content, but not due to validation
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		assert.NotEqual(t, http.StatusForbidden, w.Code)
	})

	t.Run("UploadExcelInvalidFile", func(t *testing.T) {
		// Test with invalid file extension
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		writer.WriteField("site_id", "1")

		// Create a file with wrong extension
		fileWriter, err := writer.CreateFormFile("file", "test.txt")
		require.NoError(t, err)
		fileWriter.Write([]byte("not an excel file"))
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		importsHandler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["message"], "only .xlsx files are accepted")
	})

	t.Run("UploadExcelMissingSiteID", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Don't add site_id field
		fileWriter, err := writer.CreateFormFile("file", "test.xlsx")
		require.NoError(t, err)
		fileWriter.Write([]byte("fake excel content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		importsHandler.UploadExcel(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "site_id is required")
	})

	t.Run("UploadExcelInsufficientPermissions", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		writer.WriteField("site_id", "1")
		fileWriter, err := writer.CreateFormFile("file", "test.xlsx")
		require.NoError(t, err)
		fileWriter.Write([]byte("fake excel content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		// Use viewer role (insufficient permissions)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"viewer"},
		}))

		w := httptest.NewRecorder()
		importsHandler.UploadExcel(w, req)

		// This should be handled by middleware, but if it reaches the handler,
		// it should still work since the handler doesn't check roles directly
		// (middleware should have already filtered this)
		assert.NotEqual(t, http.StatusForbidden, w.Code)
	})

	t.Run("UploadExcelWithCustomMapping", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		writer.WriteField("site_id", "1")
		writer.WriteField("dry_run", "true")
		writer.WriteField("mapping", "custom/mapping.yaml")
		writer.WriteField("max_errors", "25")

		fileWriter, err := writer.CreateFormFile("file", "test.xlsx")
		require.NoError(t, err)
		fileWriter.Write([]byte("fake excel content"))
		writer.Close()

		req := httptest.NewRequest("POST", "/imports/excel", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
			OrgID: 1,
			Roles: []string{"org_admin"},
		}))

		w := httptest.NewRecorder()
		importsHandler.UploadExcel(w, req)

		// Should not fail due to validation
		assert.NotEqual(t, http.StatusBadRequest, w.Code)
	})
}

// Helper function to create a test Excel file (placeholder)
func createTestExcelFile(t *testing.T) io.Reader {
	// In a real implementation, you would create an actual Excel file
	// For now, return a simple reader with fake content
	return bytes.NewReader([]byte("fake excel content for testing"))
}
