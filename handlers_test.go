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
