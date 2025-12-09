package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/skip2/go-qrcode"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error JSON response
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

// handleShorten handles POST /s - creates a shortened URL
func (a *App) handleShorten(w http.ResponseWriter, r *http.Request) {
	log.Info("Shorten URL requested", "method", r.Method, "path", r.URL.Path)
	var req ShortenRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("Invalid request body", "error", err, "method", r.Method)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	resp, err := a.createShortURL(&req)
	if err != nil {
		log.Error("Failed to create short URL", "error", err, "url", req.URL)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Info("URL shortened", "original", req.URL, "short_code", resp.ShortCode)
	writeJSON(w, http.StatusCreated, resp)
}

// handleShortenGET handles GET /s?u=URL - creates a shortened URL via query parameter
func (a *App) handleShortenGET(w http.ResponseWriter, r *http.Request) {
	log.Info("Shorten URL requested (GET)", "method", r.Method, "path", r.URL.Path)

	// Get URL from query parameter
	urlParam := r.URL.Query().Get("u")
	if urlParam == "" {
		log.Error("Missing URL query parameter", "method", r.Method)
		writeError(w, http.StatusBadRequest, "Missing 'u' query parameter")
		return
	}

	req := &ShortenRequest{URL: urlParam}
	resp, err := a.createShortURL(req)
	if err != nil {
		log.Error("Failed to create short URL", "error", err, "url", req.URL)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Info("URL shortened", "original", req.URL, "short_code", resp.ShortCode)
	writeJSON(w, http.StatusCreated, resp)
}

// handleRedirect handles GET /{shortened} - redirects to original URL
func (a *App) handleRedirect(w http.ResponseWriter, r *http.Request) {
	log.Info("Redirect requested", "method", r.Method, "path", r.URL.Path)
	shortCode := strings.TrimPrefix(r.URL.Path, "/")

	// Filter out special endpoints
	if shortCode == "" || shortCode == "health" || strings.HasSuffix(shortCode, "/stats") || strings.HasSuffix(shortCode, "/qr") {
		log.Info("Special endpoint filtered", "path", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// Remove trailing slash if present
	shortCode = strings.TrimSuffix(shortCode, "/")

	record, err := a.getURL(shortCode)
	if err != nil {
		log.Warn("Short code not found", "short_code", shortCode, "error", err)
		http.NotFound(w, r)
		return
	}

	// Track the click asynchronously
	go func() {
		userAgent := r.Header.Get("User-Agent")
		referer := r.Header.Get("Referer")

		if err := a.trackClick(record.ID, userAgent, referer); err != nil {
			log.Error("Failed to track click", "error", err, "url_id", record.ID)
		}
	}()

	log.Info("Redirecting", "short_code", shortCode, "original_url", record.OriginalURL)
	http.Redirect(w, r, record.OriginalURL, http.StatusMovedPermanently)
}

// handleStats handles GET /{shortened}/stats - returns URL statistics
func (a *App) handleStats(w http.ResponseWriter, r *http.Request) {
	log.Info("Stats requested", "method", r.Method, "path", r.URL.Path)
	path := strings.TrimPrefix(r.URL.Path, "/")
	shortCode := strings.TrimSuffix(path, "/stats")

	if shortCode == "" {
		log.Error("Empty short code in stats request", "path", r.URL.Path)
		writeError(w, http.StatusBadRequest, "Short code is required")
		return
	}

	stats, err := a.getStats(shortCode)
	if err != nil {
		log.Warn("Failed to get stats", "short_code", shortCode, "error", err)
		writeError(w, http.StatusNotFound, "Short code not found")
		return
	}

	log.Info("Stats retrieved", "short_code", shortCode, "clicks", stats.TotalClicks)
	writeJSON(w, http.StatusOK, stats)
}

// handleQR handles GET /{shortened}/qr - generates QR code
func (a *App) handleQR(w http.ResponseWriter, r *http.Request) {
	log.Info("QR code requested", "method", r.Method, "path", r.URL.Path)
	path := strings.TrimPrefix(r.URL.Path, "/")
	shortCode := strings.TrimSuffix(path, "/qr")

	if shortCode == "" {
		log.Error("Empty short code in QR request", "path", r.URL.Path)
		writeError(w, http.StatusBadRequest, "Short code is required")
		return
	}

	// Verify short code exists
	record, err := a.getURL(shortCode)
	if err != nil {
		log.Warn("Short code not found for QR", "short_code", shortCode, "error", err)
		writeError(w, http.StatusNotFound, "Short code not found")
		return
	}

	// Build the short URL
	shortURL := fmt.Sprintf("%s/%s", a.config.BaseURL, shortCode)

	// Generate QR code
	qr, err := qrcode.New(shortURL, qrcode.Medium)
	if err != nil {
		log.Error("Failed to generate QR code", "error", err, "url", shortURL)
		writeError(w, http.StatusInternalServerError, "Failed to generate QR code")
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day

	// Write QR code as PNG
	png, err := qr.PNG(256)
	if err != nil {
		log.Error("Failed to encode QR code as PNG", "error", err)
		writeError(w, http.StatusInternalServerError, "Failed to encode QR code")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(png)

	log.Info("QR code generated", "short_code", shortCode, "original_url", record.OriginalURL)
}
