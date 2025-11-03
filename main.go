// Package main
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sethvargo/go-envconfig"
)

var (
	Version   string = "dev"
	BuildTime string = "unknown"
	Commit    string = "none"
)

var log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

func main() {
	// Initialize configuration
	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		log.Warn("No env file found, using environment variables directly", "error", err)
	}

	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Error("Failed to process environment variables.", "error", err)
		os.Exit(1)
	}

	log.Info("Configuration loaded")

	app, err := NewApp(ctx, &cfg)
	if err != nil {
		log.Error("Failed to create application", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	appCtx, appCancel := context.WithCancel(ctx)
	defer appCancel()

	appErr := make(chan error, 1)
	go func() {
		appErr <- app.Start(appCtx)
	}()

	select {
	case sig := <-sigChan:
		log.Info("Received shutdown signal", "signal", sig.String())
		appCancel()
	case err := <-appErr:
		if err != nil {
			log.Error("Application error", "error", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.server.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", "error", err)
	} else {
		log.Info("HTTP server stopped")
	}

	if err := app.db.Close(); err != nil {
		log.Error("Database close error", "error", err)
		os.Exit(1)
	}

	log.Info("Shutdown completed. See you later!")
}

type Config struct {
	DatabaseURL string `env:"UL_DATABASE_URL, required"`
	Port        string `env:"UL_PORT, default=7000"`
}

type App struct {
	db     *sql.DB
	config *Config
	server *http.Server
}

type AppOption func(*App) error

func WithRoutes(mux *http.ServeMux) AppOption {
	return func(a *App) error {
		if mux == nil {
			return fmt.Errorf("mux cannot be nil")
		}
		a.server.Handler = mux
		return nil
	}
}

func NewApp(ctx context.Context, config *Config, opts ...AppOption) (*App, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	db, err := sql.Open("sqlite3", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify database connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection established")

	// Create app instance
	app := &App{
		db:     db,
		config: config,
		server: &http.Server{
			Addr:         ":" + config.Port,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	// Initialize database schema
	if err := app.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Apply functional options
	for _, opt := range opts {
		if err := opt(app); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// If no custom routes provided, set up default routes
	if app.server.Handler == nil {
		app.server.Handler = app.setupRoutes()
	}

	return app, nil
}

func (a *App) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","version":"%s", "buildTime":"%s", "commit":"%s"}`, Version, BuildTime, Commit)
	})

	// URL shortener endpoints
	mux.HandleFunc("POST /s", a.handleShorten)
	mux.HandleFunc("GET /{shortCode}/stats", a.handleStats)
	mux.HandleFunc("GET /{shortCode}/qr", a.handleQR)
	mux.HandleFunc("GET /{shortCode}", a.handleRedirect)

	return mux
}

func (a *App) Start(ctx context.Context) error {
	log.Info("Starting HTTP server", "address", a.server.Addr, "version", Version)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for either context cancellation or server error
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
