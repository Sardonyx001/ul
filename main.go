// Package main
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/sethvargo/go-envconfig"
)

var (
	Version   string
	BuildTime string
	Commit    string
)

var log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

func main() {
	// Initialize configuration
	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		log.Warn("No .env file found, using environment variables directly", "error", err)
	}

	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Error("Failed to process environment variables.", "error", err)
		os.Exit(1)
	}

	log.Info("Configuration loaded", "port", cfg.Port, "database_url", cfg.DatabaseURL)
	// mx := http.NewServeMux()
	// mx.HandleFunc("POST /api/v1", func(w http.ResponseWriter, r *http.Request) {
	// 	// log.Info("Request.received", "method", r.Method, "url", r.URL)
	//
	// 	fmt.Fprintf(w, "Hello, World!")
	// })
	//
	// // log.Info("Starting server on :7000")
	// panic(http.ListenAndServe(":7000", mx))
}

type Config struct {
	DatabaseURL string `env:"UL_DATABASE_URL, required"`
	Port        string `env:"UL_PORT, default=7000"`
}

type Server struct {
	db     *sql.DB
	config *Config
}

func NewServer(cfg *Config) *Server {
	if cfg == nil {
		log.Error("Configuration is nil")
		return nil
	}

	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		log.Error("Failed to connect to database", "error", err)
		return nil
	}

	return &Server{
		db:     db,
		config: cfg,
	}
}
