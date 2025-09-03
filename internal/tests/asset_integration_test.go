package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/models"
	"era-inventory-api/internal/testutil"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up test database
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	// Create test server
	server := &Server{DB: db}

	// Test creating an asset
	t.Run("CreateAsset", func(t *testing.T) {
		assetInput := models.CreateAssetRequest{
			SiteID:    1, // Assuming site ID 1 exists in test data
			AssetType: "switch",
			Name:      stringPtr("Test Switch"),
			Vendor:    stringPtr("Cisco"),
			Model:     stringPtr("C2960X"),
			Serial:    stringPtr("TEST123"),
			Status:    stringPtr("active"),
		}

		jsonData, err := json.Marshal(assetInput)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/assets", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		w := httptest.NewRecorder()
		server.createAsset(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response models.Asset
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, assetInput.Name, response.Name)
		assert.Equal(t, assetInput.AssetType, response.AssetType)
	})

	// Test listing assets
	t.Run("ListAssets", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		w := httptest.NewRecorder()
		server.listAssets(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "data")
		assert.Contains(t, response, "page")
	})

	// Test getting a specific asset
	t.Run("GetAsset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/1", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		// Set up chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		server.getAsset(w, req)

		// Should return 200 if asset exists, 404 if not
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
	})

	// Test creating a switch with subtype data
	t.Run("CreateSwitchWithSubtype", func(t *testing.T) {
		assetInput := models.CreateAssetRequest{
			SiteID:    1,
			AssetType: "switch",
			Name:      stringPtr("Test Switch with Subtype"),
			Vendor:    stringPtr("Cisco"),
			Model:     stringPtr("C2960X"),
			Serial:    stringPtr("TEST456"),
			Status:    stringPtr("active"),
			Switch: &models.CreateAssetSwitchRequest{
				PortsTotal: intPtr(48),
				POE:        boolPtr(true),
				Firmware:   stringPtr("15.2(4)S7"),
			},
		}

		jsonData, err := json.Marshal(assetInput)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/assets", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		w := httptest.NewRecorder()
		server.createAsset(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	// Test creating a VLAN with subtype data
	t.Run("CreateVLANWithSubtype", func(t *testing.T) {
		assetInput := models.CreateAssetRequest{
			SiteID:    1,
			AssetType: "vlan",
			Name:      stringPtr("Test VLAN"),
			Status:    stringPtr("active"),
			VLAN: &models.CreateAssetVLANRequest{
				VLANID:  100,
				Subnet:  stringPtr("192.168.100.0/24"),
				Gateway: stringPtr("192.168.100.1"),
				Purpose: stringPtr("Guest Network"),
			},
		}

		jsonData, err := json.Marshal(assetInput)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/assets", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		w := httptest.NewRecorder()
		server.createAsset(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	// Test listing switches
	t.Run("ListSwitches", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/switches", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		w := httptest.NewRecorder()
		server.listSwitches(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test listing VLANs
	t.Run("ListVLANs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/vlans", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		w := httptest.NewRecorder()
		server.listVLANs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test getting site asset categories
	t.Run("GetSiteAssetCategories", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sites/1/asset-categories", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.OrgIDKey, int64(1)))

		// Set up chi context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		server.getSiteAssetCategories(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
