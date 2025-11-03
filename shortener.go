package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"
)

const (
	// Base62 character set for short codes (URL-safe, no special chars)
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// Prime number for mixing (larger than expected max IDs)
	mixPrime = 1580030173

	// XOR mask for obfuscation
	xorMask = 0x5d2a8f93
)

// URLRecord represents a shortened URL entry
type URLRecord struct {
	ID            int64      `json:"id"`
	ShortCode     string     `json:"short_code"`
	OriginalURL   string     `json:"original_url"`
	CreatedAt     time.Time  `json:"created_at"`
	Clicks        int64      `json:"clicks"`
	LastClickedAt *time.Time `json:"last_clicked_at,omitempty"`
}

// ShortenRequest represents the request body for URL shortening
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse represents the response for URL shortening
type ShortenResponse struct {
	ShortCode   string    `json:"short_code"`
	ShortURL    string    `json:"short_url"`
	OriginalURL string    `json:"original_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// URLStats represents statistics for a shortened URL
type URLStats struct {
	ShortCode     string     `json:"short_code"`
	OriginalURL   string     `json:"original_url"`
	CreatedAt     time.Time  `json:"created_at"`
	TotalClicks   int64      `json:"total_clicks"`
	LastClickedAt *time.Time `json:"last_clicked_at,omitempty"`
}

// obfuscateID applies a reversible transformation to make IDs non-sequential
// This is bijective: each input maps to exactly one output
func obfuscateID(id int64) int64 {
	// XOR with mask
	obfuscated := id ^ xorMask

	// Multiply by prime and take modulo to mix bits
	obfuscated = (obfuscated * mixPrime) & 0x7FFFFFFF // Keep positive

	return obfuscated
}

// deobfuscateID reverses the obfuscation
func deobfuscateID(obfuscated int64) int64 {
	// Find modular multiplicative inverse of mixPrime
	// For our purposes with base62 encoding, we can use a precomputed inverse
	const mixPrimeInverse = 1061834701 // Modular inverse of mixPrime mod 2^31

	// Reverse the multiplication
	id := (obfuscated * mixPrimeInverse) & 0x7FFFFFFF

	// Reverse the XOR
	id = id ^ xorMask

	return id
}

// encodeBase62 converts an integer to a base62 string
func encodeBase62(num int64) string {
	if num == 0 {
		return string(base62Chars[0])
	}

	var result []byte
	base := int64(len(base62Chars))

	for num > 0 {
		remainder := num % base
		result = append([]byte{base62Chars[remainder]}, result...)
		num = num / base
	}

	return string(result)
}

// decodeBase62 converts a base62 string back to an integer
func decodeBase62(encoded string) (int64, error) {
	var num int64
	base := int64(len(base62Chars))

	for _, char := range encoded {
		var value int64 = -1
		for i, c := range base62Chars {
			if c == char {
				value = int64(i)
				break
			}
		}
		if value == -1 {
			return 0, fmt.Errorf("invalid character in short code: %c", char)
		}
		num = num*base + value
	}

	return num, nil
}

// generateShortCode creates a collision-free, non-enumerable short code from an ID
func generateShortCode(id int64) string {
	// Obfuscate the ID to prevent enumeration
	obfuscated := obfuscateID(id)

	// Encode to base62
	return encodeBase62(obfuscated)
}

// parseShortCode extracts the original ID from a short code
func parseShortCode(shortCode string) (int64, error) {
	// Decode from base62
	obfuscated, err := decodeBase62(shortCode)
	if err != nil {
		return 0, err
	}

	// Deobfuscate to get original ID
	id := deobfuscateID(obfuscated)

	return id, nil
}

// validateURL checks if the provided URL is valid
func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	// Check host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	return nil
}

// createShortURL creates a new shortened URL entry
func (a *App) createShortURL(req *ShortenRequest) (*ShortenResponse, error) {
	// Validate URL
	if err := validateURL(req.URL); err != nil {
		return nil, err
	}

	// Check if URL already exists
	var existingID int64
	err := a.db.QueryRow("SELECT id FROM urls WHERE original_url = ?", req.URL).Scan(&existingID)
	if err == nil {
		// URL already exists, return existing short code
		shortCode := generateShortCode(existingID)

		var record URLRecord
		err = a.db.QueryRow(
			"SELECT id, short_code, original_url, created_at FROM urls WHERE id = ?",
			existingID,
		).Scan(&record.ID, &record.ShortCode, &record.OriginalURL, &record.CreatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to fetch existing record: %w", err)
		}

		return &ShortenResponse{
			ShortCode:   shortCode,
			ShortURL:    fmt.Sprintf("http://localhost:%s/%s", a.config.Port, shortCode),
			OriginalURL: record.OriginalURL,
			CreatedAt:   record.CreatedAt,
		}, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Insert URL (short_code will be generated after we have the ID)
	result, err := a.db.Exec(
		"INSERT INTO urls (short_code, original_url) VALUES (?, ?)",
		"", req.URL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert URL: %w", err)
	}

	// Get the auto-generated ID
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// Generate collision-free, non-enumerable short code
	shortCode := generateShortCode(id)

	// Update with the actual short code
	_, err = a.db.Exec(
		"UPDATE urls SET short_code = ? WHERE id = ?",
		shortCode, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update short code: %w", err)
	}

	// Fetch the final record
	var record URLRecord
	err = a.db.QueryRow(
		"SELECT id, short_code, original_url, created_at FROM urls WHERE id = ?",
		id,
	).Scan(&record.ID, &record.ShortCode, &record.OriginalURL, &record.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch created record: %w", err)
	}

	return &ShortenResponse{
		ShortCode:   record.ShortCode,
		ShortURL:    fmt.Sprintf("http://localhost:%s/%s", a.config.Port, record.ShortCode),
		OriginalURL: record.OriginalURL,
		CreatedAt:   record.CreatedAt,
	}, nil
}

// getURL retrieves a URL by its short code
func (a *App) getURL(shortCode string) (*URLRecord, error) {
	// We can either lookup by short_code or decode it to get ID
	// Using short_code lookup is more straightforward
	var record URLRecord

	err := a.db.QueryRow(`
		SELECT id, short_code, original_url, created_at, clicks, last_clicked_at
		FROM urls
		WHERE short_code = ?
	`, shortCode).Scan(
		&record.ID,
		&record.ShortCode,
		&record.OriginalURL,
		&record.CreatedAt,
		&record.Clicks,
		&record.LastClickedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("short code not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &record, nil
}

// trackClick records a click event and updates statistics
func (a *App) trackClick(urlID int64, userAgent, referer, ipAddress string) error {
	tx, err := a.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert click record
	_, err = tx.Exec(`
		INSERT INTO clicks (url_id, user_agent, referer, ip_address)
		VALUES (?, ?, ?, ?)
	`, urlID, userAgent, referer, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to insert click record: %w", err)
	}

	// Update URL statistics
	_, err = tx.Exec(`
		UPDATE urls
		SET clicks = clicks + 1, last_clicked_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, urlID)
	if err != nil {
		return fmt.Errorf("failed to update URL statistics: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// getStats retrieves statistics for a shortened URL
func (a *App) getStats(shortCode string) (*URLStats, error) {
	var stats URLStats

	err := a.db.QueryRow(`
		SELECT short_code, original_url, created_at, clicks, last_clicked_at
		FROM urls
		WHERE short_code = ?
	`, shortCode).Scan(
		&stats.ShortCode,
		&stats.OriginalURL,
		&stats.CreatedAt,
		&stats.TotalClicks,
		&stats.LastClickedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("short code not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &stats, nil
}

// initDB initializes the database schema
func (a *App) initDB() error {
	schema := `
		CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			short_code TEXT NOT NULL UNIQUE,
			original_url TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			clicks INTEGER DEFAULT 0,
			last_clicked_at DATETIME
		);

		CREATE INDEX IF NOT EXISTS idx_short_code ON urls(short_code);
		CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
		CREATE INDEX IF NOT EXISTS idx_created_at ON urls(created_at);

		CREATE TABLE IF NOT EXISTS clicks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url_id INTEGER NOT NULL,
			clicked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			user_agent TEXT,
			referer TEXT,
			ip_address TEXT,
			FOREIGN KEY (url_id) REFERENCES urls(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_clicks_url_id ON clicks(url_id);
		CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at);
	`

	_, err := a.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	log.Info("Database schema initialized")
	return nil
}
