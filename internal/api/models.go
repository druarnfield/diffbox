package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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

	parseSize := func(s string) int64 {
		var n int64
		_, _ = fmt.Sscanf(s, "%d", &n)
		return n
	}

	// Build a map of filename -> aria2 status for quick lookup
	aria2ByFilename := make(map[string]*struct {
		completedLength int64
		totalLength     int64
		downloadSpeed   int64
	})
	for _, active := range activeDownloads {
		// Get filename from aria2's Files array
		if len(active.Files) > 0 && active.Files[0].Path != "" {
			filename := filepath.Base(active.Files[0].Path)
			completedLength := parseSize(active.CompletedLength)
			totalLength := parseSize(active.TotalLength)
			downloadSpeed := parseSize(active.DownloadSpeed)

			aria2ByFilename[filename] = &struct {
				completedLength int64
				totalLength     int64
				downloadSpeed   int64
			}{completedLength, totalLength, downloadSpeed}
		}
	}

	for _, model := range requiredModels {
		status := DownloadStatus{
			Name:      model.Name,
			URL:       model.URL,
			TotalSize: model.Size,
			Workflow:  model.Workflow,
		}

		filePath := filepath.Join(s.cfg.ModelsDir, model.Name)
		controlFile := filePath + ".aria2" // aria2 creates this file during download

		// FIRST: Check for .aria2 control file (most reliable indicator of in-progress download)
		if _, err := os.Stat(controlFile); err == nil {
			status.Status = "downloading"

			// Try to find this download in aria2's active list by filename
			if aria2Status, found := aria2ByFilename[model.Name]; found {
				// Use aria2's actual progress, not file size (aria2 pre-allocates!)
				status.CompletedSize = aria2Status.completedLength
				status.DownloadSpeed = aria2Status.downloadSpeed
				if aria2Status.totalLength > 0 {
					status.Progress = float64(aria2Status.completedLength) / float64(aria2Status.totalLength) * 100
				}
			} else {
				// If not found in active downloads, fall back to file size
				// This can happen if aria2 just finished but hasn't removed .aria2 yet
				if fileInfo, err := os.Stat(filePath); err == nil {
					status.CompletedSize = fileInfo.Size()
					if model.Size > 0 {
						status.Progress = float64(fileInfo.Size()) / float64(model.Size) * 100
					}
				}
			}

			downloads = append(downloads, status)
			continue
		}

		// SECOND: Check if complete file exists locally (no .aria2 file = download finished)
		fileInfo, err := os.Stat(filePath)
		if err == nil && fileInfo.Size() >= int64(float64(model.Size)*0.99) {
			status.Status = "complete"
			status.Progress = 100.0
			status.CompletedSize = fileInfo.Size()
			downloads = append(downloads, status)
			continue
		}

		// LAST: File doesn't exist or is incomplete with no active download
		if err == nil {
			// Partial file exists but no .aria2 control file
			status.Status = "downloading"
			status.CompletedSize = fileInfo.Size()
			if model.Size > 0 {
				status.Progress = float64(fileInfo.Size()) / float64(model.Size) * 100
			}
		} else {
			status.Status = "missing"
			status.Progress = 0
			status.CompletedSize = 0
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
