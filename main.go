package main

import (
	"go-system-monitor/handlers"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := ":8080"

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Route("/api", func(r chi.Router) {
		r.Get("/sensors", handlers.SensorsHandler)
	})

	slog.Info("Server listening on port", "port", port)
	if err := http.ListenAndServe(":8080", r); err != nil {
		slog.Error("Server failed: %v", err)
	}
}
