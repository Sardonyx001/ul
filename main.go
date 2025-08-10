// Package main
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	mx := http.NewServeMux()
	mx.HandleFunc("POST /api/v1", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Request.received", "method", r.Method, "url", r.URL)

		fmt.Fprintf(w, "Hello, World!")
	})

	log.Info("Starting server on :7000")
	panic(http.ListenAndServe(":7000", mx))
}
