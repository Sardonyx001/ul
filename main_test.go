package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewApp_NilConfig(t *testing.T) {
	_, err := NewApp(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil config, got nil")
	}
}

func TestNewApp_InvalidDatabaseURL(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "invalid://bad-url",
		Port:        "7000",
		BaseURL:     "http://localhost:7000",
	}

	_, err := NewApp(context.Background(), cfg)
	if err == nil {
		t.Error("Expected error for invalid database URL, got nil")
	}
}

func TestNewApp_WithRoutes(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "file::memory:?cache=shared",
		Port:        "7000",
		BaseURL:     "http://localhost:7000",
	}

	customMux := http.NewServeMux()
	customMux.HandleFunc("/custom", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("custom"))
	})

	app, err := NewApp(context.Background(), cfg, WithRoutes(customMux))
	if err != nil {
		t.Fatalf("Failed to create app with custom routes: %v", err)
	}
	defer app.db.Close()

	if app.server.Handler != customMux {
		t.Error("Expected custom mux to be set as handler")
	}
}

func TestWithRoutes_NilMux(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "file::memory:?cache=shared",
		Port:        "7000",
		BaseURL:     "http://localhost:7000",
	}

	_, err := NewApp(context.Background(), cfg, WithRoutes(nil))
	if err == nil {
		t.Error("Expected error for nil mux, got nil")
	}
}

func TestSetupRoutes(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	mux := app.setupRoutes()

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d for health check, got %d", http.StatusOK, rec.Code)
	}
}

func TestSetupRoutes_HealthEndpoint(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	mux := app.setupRoutes()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestHealthEndpoint_JSONResponse(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler := app.setupRoutes()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !contains(body, "status") || !contains(body, "ok") {
		t.Errorf("Expected health response to contain status ok, got: %s", body)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
