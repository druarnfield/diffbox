package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/druarnfield/diffbox/internal/db"
	"github.com/google/uuid"
)

// I2V Request
type I2VRequest struct {
	Prompt            string   `json:"prompt"`
	NegativePrompt    string   `json:"negative_prompt"`
	InputImage        string   `json:"input_image"` // base64 or path
	Seed              *int     `json:"seed"`
	Height            int      `json:"height"`
	Width             int      `json:"width"`
	NumFrames         int      `json:"num_frames"`
	NumInferenceSteps int      `json:"num_inference_steps"`
	CFGScale          float64  `json:"cfg_scale"`
	DenoisingStrength float64  `json:"denoising_strength"`
	CameraDirection   string   `json:"camera_direction"`
	CameraSpeed       float64  `json:"camera_speed"`
	MotionBucketID    *int     `json:"motion_bucket_id"`
	LoRAs             []string `json:"loras"`
	Tiled             bool     `json:"tiled"`
	TileSize          []int    `json:"tile_size"`
}

// SVI Request
type SVIRequest struct {
	I2VRequest
	Prompts         []string `json:"prompts"`
	NumClips        int      `json:"num_clips"`
	NumMotionFrames int      `json:"num_motion_frames"`
	InfiniteMode    bool     `json:"infinite_mode"`
}

// Qwen Request
type QwenRequest struct {
	Prompt            string   `json:"prompt"`
	NegativePrompt    string   `json:"negative_prompt"`
	EditImages        []string `json:"edit_images"`  // base64 or paths
	InpaintMask       string   `json:"inpaint_mask"` // base64
	Seed              *int     `json:"seed"`
	Height            int      `json:"height"`
	Width             int      `json:"width"`
	NumInferenceSteps int      `json:"num_inference_steps"`
	CFGScale          float64  `json:"cfg_scale"`
	DenoisingStrength float64  `json:"denoising_strength"`
	Mode              string   `json:"mode"` // "generate", "edit", "inpaint"
	ControlNet        string   `json:"controlnet"`
	ControlNetScale   float64  `json:"controlnet_scale"`
	LoRAs             []string `json:"loras"`
}

// Job Response
type JobResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (s *Server) handleI2VSubmit(w http.ResponseWriter, r *http.Request) {
	var req I2VRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("I2V: Failed to decode request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log request details (without full image data)
	log.Printf("I2V: Received request - prompt=%q, image_len=%d bytes", req.Prompt, len(req.InputImage))

	// Set defaults
	if req.Height == 0 {
		req.Height = 480
	}
	if req.Width == 0 {
		req.Width = 832
	}
	if req.NumFrames == 0 {
		req.NumFrames = 81
	}
	if req.NumInferenceSteps == 0 {
		req.NumInferenceSteps = 50
	}
	if req.CFGScale == 0 {
		req.CFGScale = 5.0
	}
	if req.DenoisingStrength == 0 {
		req.DenoisingStrength = 1.0
	}

	// Create job
	jobID := uuid.New().String()

	// Persist job to database
	paramsJSON, err := json.Marshal(req)
	if err != nil {
		log.Printf("I2V: Failed to serialize params for job %s: %v", jobID, err)
		http.Error(w, "Failed to serialize params", http.StatusInternalServerError)
		return
	}

	dbJob := &db.Job{
		ID:     jobID,
		Type:   "i2v",
		Status: "pending",
		Params: string(paramsJSON),
	}
	if err := s.db.CreateJob(dbJob); err != nil {
		log.Printf("I2V: Failed to persist job %s: %v", jobID, err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	// Queue job
	job := map[string]interface{}{
		"id":     jobID,
		"type":   "i2v",
		"params": req,
		"status": "pending",
	}

	if err := s.queue.Enqueue("jobs", job); err != nil {
		log.Printf("I2V: Failed to enqueue job %s: %v", jobID, err)
		http.Error(w, "Failed to queue job", http.StatusInternalServerError)
		return
	}

	log.Printf("I2V: Job %s queued successfully", jobID)
	// Return job ID
	json.NewEncoder(w).Encode(JobResponse{
		ID:     jobID,
		Status: "pending",
	})
}

func (s *Server) handleSVISubmit(w http.ResponseWriter, r *http.Request) {
	var req SVIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Height == 0 {
		req.Height = 480
	}
	if req.Width == 0 {
		req.Width = 832
	}
	if req.NumFrames == 0 {
		req.NumFrames = 81
	}
	if req.NumInferenceSteps == 0 {
		req.NumInferenceSteps = 50
	}
	if req.CFGScale == 0 {
		req.CFGScale = 5.0
	}
	if req.NumClips == 0 {
		req.NumClips = 10
	}
	if req.NumMotionFrames == 0 {
		req.NumMotionFrames = 5
	}

	// Create job
	jobID := uuid.New().String()

	// Persist job to database
	paramsJSON, err := json.Marshal(req)
	if err != nil {
		log.Printf("SVI: Failed to serialize params for job %s: %v", jobID, err)
		http.Error(w, "Failed to serialize params", http.StatusInternalServerError)
		return
	}

	dbJob := &db.Job{
		ID:     jobID,
		Type:   "svi",
		Status: "pending",
		Params: string(paramsJSON),
	}
	if err := s.db.CreateJob(dbJob); err != nil {
		log.Printf("SVI: Failed to persist job %s: %v", jobID, err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	// Queue job
	job := map[string]interface{}{
		"id":     jobID,
		"type":   "svi",
		"params": req,
		"status": "pending",
	}

	if err := s.queue.Enqueue("jobs", job); err != nil {
		log.Printf("SVI: Failed to enqueue job %s: %v", jobID, err)
		http.Error(w, "Failed to queue job", http.StatusInternalServerError)
		return
	}

	log.Printf("SVI: Job %s queued successfully", jobID)
	json.NewEncoder(w).Encode(JobResponse{
		ID:     jobID,
		Status: "pending",
	})
}

func (s *Server) handleQwenSubmit(w http.ResponseWriter, r *http.Request) {
	var req QwenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Height == 0 {
		req.Height = 1024
	}
	if req.Width == 0 {
		req.Width = 1024
	}
	if req.NumInferenceSteps == 0 {
		req.NumInferenceSteps = 30
	}
	if req.CFGScale == 0 {
		req.CFGScale = 4.0
	}
	if req.DenoisingStrength == 0 {
		req.DenoisingStrength = 1.0
	}
	if req.Mode == "" {
		req.Mode = "generate"
	}

	// Create job
	jobID := uuid.New().String()

	// Persist job to database
	paramsJSON, err := json.Marshal(req)
	if err != nil {
		log.Printf("Qwen: Failed to serialize params for job %s: %v", jobID, err)
		http.Error(w, "Failed to serialize params", http.StatusInternalServerError)
		return
	}

	dbJob := &db.Job{
		ID:     jobID,
		Type:   "qwen",
		Status: "pending",
		Params: string(paramsJSON),
	}
	if err := s.db.CreateJob(dbJob); err != nil {
		log.Printf("Qwen: Failed to persist job %s: %v", jobID, err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	// Queue job
	job := map[string]interface{}{
		"id":     jobID,
		"type":   "qwen",
		"params": req,
		"status": "pending",
	}

	if err := s.queue.Enqueue("jobs", job); err != nil {
		log.Printf("Qwen: Failed to enqueue job %s: %v", jobID, err)
		http.Error(w, "Failed to queue job", http.StatusInternalServerError)
		return
	}

	log.Printf("Qwen: Job %s queued successfully", jobID)
	json.NewEncoder(w).Encode(JobResponse{
		ID:     jobID,
		Status: "pending",
	})
}
