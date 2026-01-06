package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/druarnfield/diffbox/internal/db"
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
	dbJobs, err := s.db.ListJobs(100)
	if err != nil {
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	jobs := make([]Job, len(dbJobs))
	for i, dbJob := range dbJobs {
		jobs[i] = dbJobToAPIJob(dbJob)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	dbJob, err := s.db.GetJob(jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	job := dbJobToAPIJob(dbJob)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	// TODO: Implement job cancellation
	_ = jobID

	w.WriteHeader(http.StatusNoContent)
}

// dbJobToAPIJob converts a database Job to an API Job
func dbJobToAPIJob(dbJob *db.Job) Job {
	job := Job{
		ID:        dbJob.ID,
		Type:      dbJob.Type,
		Status:    dbJob.Status,
		Progress:  dbJob.Progress,
		Stage:     dbJob.Stage,
		Error:     dbJob.Error,
		CreatedAt: dbJob.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: dbJob.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Parse params JSON string into map
	if dbJob.Params != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(dbJob.Params), &params); err == nil {
			job.Params = params
		}
	}
	if job.Params == nil {
		job.Params = make(map[string]interface{})
	}

	// Convert output path to JobOutput struct
	if dbJob.Output != "" {
		outputType := "video"
		// Check for image extensions
		lowerOutput := strings.ToLower(dbJob.Output)
		if strings.HasSuffix(lowerOutput, ".png") ||
			strings.HasSuffix(lowerOutput, ".jpg") ||
			strings.HasSuffix(lowerOutput, ".jpeg") ||
			strings.HasSuffix(lowerOutput, ".webp") {
			outputType = "image"
		}
		job.Output = &JobOutput{
			Type: outputType,
			Path: dbJob.Output,
		}
	}

	return job
}
