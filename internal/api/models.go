package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Model struct {
	ID           string   `json:"id"`
	Source       string   `json:"source"` // "huggingface" or "civitai"
	SourceID     string   `json:"source_id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "checkpoint", "lora", "vae", "controlnet"
	BaseModel    string   `json:"base_model"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	Downloads    int      `json:"downloads"`
	Rating       float64  `json:"rating"`
	ThumbnailURL string   `json:"thumbnail_url"`
	LocalPath    string   `json:"local_path,omitempty"`
	Pinned       bool     `json:"pinned"`
}

type ModelsResponse struct {
	Models     []Model `json:"models"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
}

func (s *Server) handleSearchModels(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	modelType := r.URL.Query().Get("type")
	baseModel := r.URL.Query().Get("base")

	// TODO: Implement search using Bleve
	_ = query
	_ = modelType
	_ = baseModel

	response := ModelsResponse{
		Models:   []Model{},
		Total:    0,
		Page:     1,
		PageSize: 20,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleListLocalModels(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement local model listing
	models := []Model{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")
	id := chi.URLParam(r, "id")

	// TODO: Implement model fetching
	model := Model{
		ID:     source + ":" + id,
		Source: source,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model)
}

func (s *Server) handleDownloadModel(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")
	id := chi.URLParam(r, "id")

	// TODO: Implement download via aria2
	_ = source
	_ = id

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "downloading",
	})
}

func (s *Server) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")
	id := chi.URLParam(r, "id")

	// TODO: Implement model deletion
	_ = source
	_ = id

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement download listing from aria2
	downloads := []map[string]interface{}{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(downloads)
}

func (s *Server) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	downloadID := chi.URLParam(r, "id")

	// TODO: Implement download cancellation via aria2
	_ = downloadID

	w.WriteHeader(http.StatusNoContent)
}
