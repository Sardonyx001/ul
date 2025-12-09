package main

import (
	"context"
	"testing"
	"time"
)

func TestObfuscateID(t *testing.T) {
	testCases := []struct {
		id   int64
		name string
	}{
		{1, "first ID"},
		{100, "hundredth ID"},
		{999999, "large ID"},
		{0, "zero ID"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			obfuscated := obfuscateID(tc.id)
			if obfuscated == tc.id {
				t.Errorf("Obfuscated ID should differ from original")
			}
			if obfuscated < 0 {
				t.Errorf("Obfuscated ID should be positive, got %d", obfuscated)
			}
		})
	}
}

func TestEncodeBase62(t *testing.T) {
	testCases := []struct {
		num      int64
		expected string
		name     string
	}{
		{0, "0", "zero"},
		{1, "1", "one"},
		{61, "z", "max single char"},
		{62, "10", "base overflow"},
		{3844, "100", "large number"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := encodeBase62(tc.num)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestEncodeBase62_LargeNumbers(t *testing.T) {
	// Test with very large IDs
	largeID := int64(999999999)
	result := encodeBase62(largeID)
	if len(result) == 0 {
		t.Error("Expected non-empty result for large number")
	}
}

func TestGenerateShortCode(t *testing.T) {
	// Test that sequential IDs produce different short codes
	codes := make(map[string]bool)
	for i := int64(1); i <= 100; i++ {
		code := generateShortCode(i)
		if codes[code] {
			t.Errorf("Duplicate short code generated: %s", code)
		}
		codes[code] = true
		if len(code) == 0 {
			t.Error("Generated empty short code")
		}
	}
}

func TestValidateURL(t *testing.T) {
	testCases := []struct {
		url       string
		shouldErr bool
		name      string
	}{
		{"https://www.example.com", false, "valid HTTPS URL"},
		{"http://example.com", false, "valid HTTP URL"},
		{"", true, "empty URL"},
		{"not-a-url", true, "invalid format"},
		{"ftp://example.com", true, "invalid scheme"},
		{"https://", true, "missing host"},
		{"://example.com", true, "missing scheme"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateURL(tc.url)
			if tc.shouldErr && err == nil {
				t.Errorf("Expected error for URL '%s', got nil", tc.url)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Expected no error for URL '%s', got: %v", tc.url, err)
			}
		})
	}
}

func TestCreateShortURL(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Test creating new URL
	req := &ShortenRequest{URL: "https://www.example.com/create-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	if resp.ShortCode == "" {
		t.Error("Expected non-empty short code")
	}

	if resp.OriginalURL != req.URL {
		t.Errorf("Expected original URL '%s', got '%s'", req.URL, resp.OriginalURL)
	}

	if resp.ShortURL == "" {
		t.Error("Expected non-empty short URL")
	}
}

func TestCreateShortURL_InvalidURL(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	testCases := []struct {
		url  string
		name string
	}{
		{"", "empty URL"},
		{"not-a-url", "invalid format"},
		{"ftp://example.com", "invalid scheme"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &ShortenRequest{URL: tc.url}
			_, err := app.createShortURL(req)
			if err == nil {
				t.Errorf("Expected error for URL '%s', got nil", tc.url)
			}
		})
	}
}

func TestCreateShortURL_Idempotent(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	url := "https://www.example.com/idempotent-test"

	// Create first time
	req1 := &ShortenRequest{URL: url}
	resp1, err := app.createShortURL(req1)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	// Create second time with same URL
	req2 := &ShortenRequest{URL: url}
	resp2, err := app.createShortURL(req2)
	if err != nil {
		t.Fatalf("Failed to create short URL second time: %v", err)
	}

	// Should return the same short code
	if resp1.ShortCode != resp2.ShortCode {
		t.Errorf("Expected same short code, got '%s' and '%s'", resp1.ShortCode, resp2.ShortCode)
	}

	// Verify both have the same created_at timestamp
	if !resp1.CreatedAt.Equal(resp2.CreatedAt) {
		t.Error("Expected same created_at for idempotent requests")
	}
}

func TestGetURL(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL first
	req := &ShortenRequest{URL: "https://www.example.com/get-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	// Retrieve it
	record, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}

	if record.ShortCode != resp.ShortCode {
		t.Errorf("Expected short code '%s', got '%s'", resp.ShortCode, record.ShortCode)
	}

	if record.OriginalURL != req.URL {
		t.Errorf("Expected original URL '%s', got '%s'", req.URL, record.OriginalURL)
	}

	if record.Clicks != 0 {
		t.Errorf("Expected 0 clicks, got %d", record.Clicks)
	}
}

func TestGetURL_NotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	_, err := app.getURL("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent short code, got nil")
	}

	if err.Error() != "short code not found" {
		t.Errorf("Expected 'short code not found' error, got: %v", err)
	}
}

func TestGetURL_EmptyShortCode(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	_, err := app.getURL("")
	if err == nil {
		t.Error("Expected error for empty short code, got nil")
	}
}

func TestTrackClick(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL
	req := &ShortenRequest{URL: "https://www.example.com/track-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	// Get the URL to find its ID
	record, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}

	// Track a click
	userAgent := "Test-Agent/1.0"
	referer := "https://test.com"
	err = app.trackClick(record.ID, userAgent, referer)
	if err != nil {
		t.Fatalf("Failed to track click: %v", err)
	}

	// Verify the click was tracked
	updatedRecord, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get updated URL: %v", err)
	}

	if updatedRecord.Clicks != 1 {
		t.Errorf("Expected 1 click, got %d", updatedRecord.Clicks)
	}

	if updatedRecord.LastClickedAt == nil {
		t.Error("Expected last_clicked_at to be set")
	}
}

func TestTrackClick_MultipleClicks(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL
	req := &ShortenRequest{URL: "https://www.example.com/multi-click-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	record, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}

	// Track multiple clicks
	for i := 0; i < 5; i++ {
		err = app.trackClick(record.ID, "Test-Agent", "https://test.com")
		if err != nil {
			t.Fatalf("Failed to track click %d: %v", i+1, err)
		}
	}

	// Verify all clicks were tracked
	updatedRecord, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get updated URL: %v", err)
	}

	if updatedRecord.Clicks != 5 {
		t.Errorf("Expected 5 clicks, got %d", updatedRecord.Clicks)
	}
}

func TestTrackClick_EmptyUserAgent(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL
	req := &ShortenRequest{URL: "https://www.example.com/empty-ua-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	record, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}

	// Track click with empty user agent
	err = app.trackClick(record.ID, "", "")
	if err != nil {
		t.Fatalf("Failed to track click with empty user agent: %v", err)
	}

	// Verify click was tracked
	updatedRecord, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get updated URL: %v", err)
	}

	if updatedRecord.Clicks != 1 {
		t.Errorf("Expected 1 click, got %d", updatedRecord.Clicks)
	}
}

func TestGetStats(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL
	req := &ShortenRequest{URL: "https://www.example.com/stats-func-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	// Track some clicks
	record, err := app.getURL(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}

	for i := 0; i < 3; i++ {
		err = app.trackClick(record.ID, "Test-Agent", "https://test.com")
		if err != nil {
			t.Fatalf("Failed to track click: %v", err)
		}
	}

	// Get stats
	stats, err := app.getStats(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.ShortCode != resp.ShortCode {
		t.Errorf("Expected short code '%s', got '%s'", resp.ShortCode, stats.ShortCode)
	}

	if stats.OriginalURL != req.URL {
		t.Errorf("Expected original URL '%s', got '%s'", req.URL, stats.OriginalURL)
	}

	if stats.TotalClicks != 3 {
		t.Errorf("Expected 3 total clicks, got %d", stats.TotalClicks)
	}

	if stats.LastClickedAt == nil {
		t.Error("Expected last_clicked_at to be set")
	}
}

func TestGetStats_NotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	_, err := app.getStats("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent short code, got nil")
	}
}

func TestGetStats_NoClicks(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL but don't track any clicks
	req := &ShortenRequest{URL: "https://www.example.com/no-clicks-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	// Get stats
	stats, err := app.getStats(resp.ShortCode)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalClicks != 0 {
		t.Errorf("Expected 0 total clicks, got %d", stats.TotalClicks)
	}

	if stats.LastClickedAt != nil {
		t.Error("Expected last_clicked_at to be nil")
	}
}

func TestInitDB(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "file::memory:?cache=shared",
		Port:        "7000",
		BaseURL:     "http://localhost:7000",
	}

	app, err := NewApp(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.db.Close()

	// Verify tables were created by attempting to insert
	_, err = app.db.Exec("INSERT INTO urls (short_code, original_url) VALUES (?, ?)", "test", "https://example.com")
	if err != nil {
		t.Errorf("Failed to insert into urls table: %v", err)
	}

	// Verify clicks table
	_, err = app.db.Exec("INSERT INTO clicks (url_id, user_agent, referer) VALUES (?, ?, ?)", 1, "test", "test")
	if err != nil {
		t.Errorf("Failed to insert into clicks table: %v", err)
	}
}

func TestValidateURL_EdgeCases(t *testing.T) {
	testCases := []struct {
		url  string
		name string
	}{
		{"", "empty URL"},
		{"invalid-url", "invalid URL format"},
		{"http://", "http with no host"},
		{"https://", "https with no host"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateURL(tc.url)
			if err == nil {
				t.Errorf("Expected validation error for %s", tc.name)
			}
		})
	}
}

func TestURLRecordTimestamps(t *testing.T) {
	app := setupTestApp(t)
	defer app.db.Close()

	// Create a URL
	req := &ShortenRequest{URL: "https://www.example.com/timestamp-test"}
	resp, err := app.createShortURL(req)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	// Check that created_at is set
	if resp.CreatedAt.IsZero() {
		t.Error("Expected created_at to be set")
	}

	// Check that created_at is recent (within last minute)
	if time.Since(resp.CreatedAt) > time.Minute {
		t.Error("Created timestamp is too old")
	}
}
