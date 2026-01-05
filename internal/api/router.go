package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/druarnfield/diffbox/internal/config"
	"github.com/druarnfield/diffbox/internal/db"
	"github.com/druarnfield/diffbox/internal/queue"
)

type Server struct {
	cfg   *config.Config
	db    *db.DB
	queue queue.Queue
	hub   *WebSocketHub
}

func NewRouter(cfg *config.Config, database *db.DB, q queue.Queue) http.Handler {
	s := &Server{
		cfg:   cfg,
		db:    database,
		queue: q,
		hub:   NewWebSocketHub(),
	}

	// Start WebSocket hub
	go s.hub.Run()

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Workflows
		r.Route("/workflows", func(r chi.Router) {
			r.Post("/i2v", s.handleI2VSubmit)
			r.Post("/svi", s.handleSVISubmit)
			r.Post("/qwen", s.handleQwenSubmit)
		})

		// Jobs
		r.Route("/jobs", func(r chi.Router) {
			r.Get("/", s.handleListJobs)
			r.Get("/{id}", s.handleGetJob)
			r.Delete("/{id}", s.handleCancelJob)
		})

		// Models
		r.Route("/models", func(r chi.Router) {
			r.Get("/", s.handleSearchModels)
			r.Get("/local", s.handleListLocalModels)
			r.Get("/{source}/{id}", s.handleGetModel)
			r.Post("/{source}/{id}/download", s.handleDownloadModel)
			r.Delete("/{source}/{id}", s.handleDeleteModel)
		})

		// Downloads
		r.Route("/downloads", func(r chi.Router) {
			r.Get("/", s.handleListDownloads)
			r.Delete("/{id}", s.handleCancelDownload)
		})

		// Config
		r.Route("/config", func(r chi.Router) {
			r.Get("/", s.handleExportConfig)
			r.Post("/", s.handleImportConfig)
			r.Get("/tokens", s.handleGetTokenStatus)
			r.Put("/tokens", s.handleUpdateTokens)
		})

		// Health
		r.Get("/health", s.handleHealth)
	})

	// WebSocket
	r.Get("/ws", s.handleWebSocket)

	// Static files (frontend)
	fileServer := http.FileServer(http.Dir(cfg.StaticDir))
	r.Handle("/*", http.StripPrefix("/", fileServer))

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
