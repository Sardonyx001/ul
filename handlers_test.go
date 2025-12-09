package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestApp(t *testing.T) *App {
	t.Helper()

	cfg := &Config{
		DatabaseURL: "file::memory:?cache=shared",
		Port:        "7000",
		BaseURL:     "http://localhost:7000",
	}

	app, err := NewApp(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create test app: %v", err)
	}

	return app
}

func TestHandleShortenPOST(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	reqBody := `{"url":"https://www.example.com"}`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.handleShorten(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.OriginalURL != "https://www.example.com" {
		t.Errorf("Expected original URL 'https://www.example.com', got '%s'", resp.OriginalURL)
	}

	if resp.ShortCode == "" {
		t.Error("Expected non-empty short code")
	}
}

func TestHandleShortenPOST_InvalidJSON(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	reqBody := `{invalid json`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.handleShorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleShortenPOST_InvalidURL(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	reqBody := `{"url":"not-a-valid-url"}`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.handleShorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleShortenGET(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/s?u=https://www.example.com", nil)
	rec := httptest.NewRecorder()

	app.handleShortenGET(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.OriginalURL != "https://www.example.com" {
		t.Errorf("Expected original URL 'https://www.example.com', got '%s'", resp.OriginalURL)
	}

	if resp.ShortCode == "" {
		t.Error("Expected non-empty short code")
	}
}

func TestHandleShortenGET_MissingParam(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/s", nil)
	rec := httptest.NewRecorder()

	app.handleShortenGET(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if !strings.Contains(errResp.Error, "Missing 'u' query parameter") {
		t.Errorf("Expected error about missing parameter, got '%s'", errResp.Error)
	}
}

func TestHandleShortenGET_InvalidURL(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/s?u=not-a-valid-url", nil)
	rec := httptest.NewRecorder()

	app.handleShortenGET(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleShortenGET_SameURLReturnsSameCode(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	testURL := "https://www.example.com/test"

	// First request
	req1 := httptest.NewRequest("GET", "/s?u="+testURL, nil)
	rec1 := httptest.NewRecorder()
	app.handleShortenGET(rec1, req1)

	var resp1 ShortenResponse
	if err := json.NewDecoder(rec1.Body).Decode(&resp1); err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	// Second request with same URL
	req2 := httptest.NewRequest("GET", "/s?u="+testURL, nil)
	rec2 := httptest.NewRecorder()
	app.handleShortenGET(rec2, req2)

	var resp2 ShortenResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("Failed to decode second response: %v", err)
	}

	// Both should return the same short code
	if resp1.ShortCode != resp2.ShortCode {
		t.Errorf("Expected same short code, got '%s' and '%s'", resp1.ShortCode, resp2.ShortCode)
	}
}

func TestHandleRedirect(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a shortened URL first
	reqBody := `{"url":"https://www.example.com/redirect-test"}`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.handleShorten(rec, req)

	var resp ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Test redirect
	redirectReq := httptest.NewRequest("GET", "/"+resp.ShortCode, nil)
	redirectReq.Header.Set("User-Agent", "Test-Agent")
	redirectReq.Header.Set("Referer", "https://test.com")
	redirectRec := httptest.NewRecorder()

	app.handleRedirect(redirectRec, redirectReq)

	if redirectRec.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status %d, got %d", http.StatusMovedPermanently, redirectRec.Code)
	}

	location := redirectRec.Header().Get("Location")
	if location != "https://www.example.com/redirect-test" {
		t.Errorf("Expected redirect to 'https://www.example.com/redirect-test', got '%s'", location)
	}
}

func TestHandleRedirect_WithTrailingSlash(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a shortened URL
	reqBody := `{"url":"https://www.example.com/trailing-slash-test"}`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.handleShorten(rec, req)

	var resp ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Test redirect with trailing slash
	redirectReq := httptest.NewRequest("GET", "/"+resp.ShortCode+"/", nil)
	redirectRec := httptest.NewRecorder()

	app.handleRedirect(redirectRec, redirectReq)

	if redirectRec.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status %d, got %d", http.StatusMovedPermanently, redirectRec.Code)
	}

	location := redirectRec.Header().Get("Location")
	if location != "https://www.example.com/trailing-slash-test" {
		t.Errorf("Expected redirect to work with trailing slash, got '%s'", location)
	}
}

func TestHandleRedirect_NotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rec := httptest.NewRecorder()

	app.handleRedirect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleRedirect_SpecialEndpoints(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	testCases := []struct {
		path string
		name string
	}{
		{"/", "root"},
		{"/health", "health"},
		{"/test/stats", "stats suffix"},
		{"/test/qr", "qr suffix"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()

			app.handleRedirect(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("Expected status %d for %s, got %d", http.StatusNotFound, tc.path, rec.Code)
			}
		})
	}
}

func TestHandleStats(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a shortened URL
	reqBody := `{"url":"https://www.example.com/stats-test"}`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.handleShorten(rec, req)

	var resp ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Simulate a click
	redirectReq := httptest.NewRequest("GET", "/"+resp.ShortCode, nil)
	redirectRec := httptest.NewRecorder()
	app.handleRedirect(redirectRec, redirectReq)

	// Give async click tracking a moment to complete
	// Note: In production tests, you might use synchronous tracking or wait groups

	// Get stats
	statsReq := httptest.NewRequest("GET", "/"+resp.ShortCode+"/stats", nil)
	statsRec := httptest.NewRecorder()
	app.handleStats(statsRec, statsReq)

	if statsRec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, statsRec.Code)
	}

	var stats URLStats
	if err := json.NewDecoder(statsRec.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode stats: %v", err)
	}

	if stats.ShortCode != resp.ShortCode {
		t.Errorf("Expected short code '%s', got '%s'", resp.ShortCode, stats.ShortCode)
	}

	if stats.OriginalURL != "https://www.example.com/stats-test" {
		t.Errorf("Expected original URL 'https://www.example.com/stats-test', got '%s'", stats.OriginalURL)
	}
}

func TestHandleStats_NotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/nonexistent/stats", nil)
	rec := httptest.NewRecorder()

	app.handleStats(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleStats_EmptyShortCode(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "//stats", nil)
	rec := httptest.NewRecorder()

	app.handleStats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleQR(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a shortened URL
	reqBody := `{"url":"https://www.example.com/qr-test"}`
	req := httptest.NewRequest("POST", "/s", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.handleShorten(rec, req)

	var resp ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Get QR code
	qrReq := httptest.NewRequest("GET", "/"+resp.ShortCode+"/qr", nil)
	qrRec := httptest.NewRecorder()
	app.handleQR(qrRec, qrReq)

	if qrRec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, qrRec.Code)
	}

	contentType := qrRec.Header().Get("Content-Type")
	if contentType != "image/png" {
		t.Errorf("Expected Content-Type 'image/png', got '%s'", contentType)
	}

	cacheControl := qrRec.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=86400" {
		t.Errorf("Expected Cache-Control 'public, max-age=86400', got '%s'", cacheControl)
	}

	// Check that we got PNG data (PNG magic number: 89 50 4E 47)
	body := qrRec.Body.Bytes()
	if len(body) < 4 {
		t.Fatal("QR code response body too short")
	}

	if body[0] != 0x89 || body[1] != 0x50 || body[2] != 0x4E || body[3] != 0x47 {
		t.Error("Response does not appear to be a valid PNG image")
	}
}

func TestHandleQR_NotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/nonexistent/qr", nil)
	rec := httptest.NewRecorder()

	app.handleQR(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleQR_EmptyShortCode(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "//qr", nil)
	rec := httptest.NewRecorder()

	app.handleQR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
