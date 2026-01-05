package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Job struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Status    string                 `json:"status"`
	Progress  float64                `json:"progress"`
	Stage     string                 `json:"stage"`
	Params    map[string]interface{} `json:"params"`
	Output    *JobOutput             `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

type JobOutput struct {
	Type   string `json:"type"` // "video" or "image"
	Path   string `json:"path"`
	Frames int    `json:"frames,omitempty"`
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement job listing from database
	jobs := []Job{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	// TODO: Implement job fetching from database
	job := Job{
		ID:     jobID,
		Status: "pending",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	// TODO: Implement job cancellation
	_ = jobID

	w.WriteHeader(http.StatusNoContent)
}
