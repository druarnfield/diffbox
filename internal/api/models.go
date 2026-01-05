package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/druarnfield/diffbox/internal/aria2"
	"github.com/druarnfield/diffbox/internal/models"
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

type DownloadStatus struct {
	Name            string  `json:"name"`
	URL             string  `json:"url"`
	Status          string  `json:"status"` // "complete", "downloading", "queued", "missing"
	Progress        float64 `json:"progress"`
	TotalSize       int64   `json:"total_size"`
	CompletedSize   int64   `json:"completed_size"`
	DownloadSpeed   int64   `json:"download_speed"`
	Workflow        string  `json:"workflow"`
}

func (s *Server) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	requiredModels := models.RequiredModels()
	downloads := make([]DownloadStatus, 0, len(requiredModels))

	// Get all active downloads from aria2
	activeDownloads, _ := s.aria2Client.TellActive()

	// Create map of active downloads by filename
	activeByName := make(map[string]*aria2.DownloadStatus)
	for i := range activeDownloads {
		// Extract filename from download (aria2 doesn't store it directly)
		activeByName[activeDownloads[i].GID] = &activeDownloads[i]
	}

	for _, model := range requiredModels {
		status := DownloadStatus{
			Name:      model.Name,
			URL:       model.URL,
			TotalSize: model.Size,
			Workflow:  model.Workflow,
		}

		// Check if file exists locally
		filePath := filepath.Join(s.cfg.ModelsDir, model.Name)
		fileInfo, err := os.Stat(filePath)

		if err == nil {
			// File exists - check if complete
			if fileInfo.Size() >= int64(float64(model.Size)*0.99) {
				status.Status = "complete"
				status.Progress = 100.0
				status.CompletedSize = fileInfo.Size()
			} else {
				// File exists but incomplete - might be downloading
				status.Status = "downloading"
				status.CompletedSize = fileInfo.Size()
				status.Progress = float64(fileInfo.Size()) / float64(model.Size) * 100
			}
		} else {
			// File doesn't exist - check if downloading
			found := false
			for _, active := range activeDownloads {
				// Match by checking if the download is for this model
				// (we'd need to track GID to filename mapping better)
				if active.Status == "active" || active.Status == "waiting" {
					found = true
					parseSize := func(s string) int64 {
						var n int64
						_, _ = fmt.Sscanf(s, "%d", &n)
						return n
					}

					total := parseSize(active.TotalLength)
					completed := parseSize(active.CompletedLength)
					speed := parseSize(active.DownloadSpeed)

					status.Status = active.Status
					status.TotalSize = total
					status.CompletedSize = completed
					status.DownloadSpeed = speed
					if total > 0 {
						status.Progress = float64(completed) / float64(total) * 100
					}
					break
				}
			}

			if !found {
				status.Status = "missing"
				status.Progress = 0
				status.CompletedSize = 0
			}
		}

		downloads = append(downloads, status)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(downloads)
}

func (s *Server) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	downloadID := chi.URLParam(r, "id")

	// TODO: Implement download cancellation via aria2
	_ = downloadID

	w.WriteHeader(http.StatusNoContent)
}
