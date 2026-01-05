package api

import (
	"encoding/json"
	"net/http"
)

type UserConfig struct {
	Version  string                 `json:"version"`
	Tokens   TokenConfig            `json:"tokens"`
	Defaults map[string]interface{} `json:"defaults"`
	Presets  []Preset               `json:"presets"`
	Models   ModelConfig            `json:"models"`
}

type TokenConfig struct {
	HuggingFace string `json:"huggingface,omitempty"`
	Civitai     string `json:"civitai,omitempty"`
}

type Preset struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Workflow string                 `json:"workflow"`
	Params   map[string]interface{} `json:"params"`
}

type ModelConfig struct {
	Base       []string `json:"base"`
	LoRA       []string `json:"lora"`
	ControlNet []string `json:"controlnet"`
	VAE        []string `json:"vae"`
}

type TokenStatus struct {
	HuggingFace bool `json:"huggingface"`
	Civitai     bool `json:"civitai"`
}

func (s *Server) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	// TODO: Build config from database
	config := UserConfig{
		Version: "1.0",
		Tokens:  TokenConfig{},
		Defaults: map[string]interface{}{
			"i2v": map[string]interface{}{
				"num_inference_steps": 50,
				"cfg_scale":           5.0,
				"height":              480,
				"width":               832,
				"num_frames":          81,
			},
			"svi": map[string]interface{}{
				"num_inference_steps": 50,
				"cfg_scale":           5.0,
				"num_motion_frames":   5,
				"clips":               10,
			},
			"qwen": map[string]interface{}{
				"num_inference_steps": 30,
				"cfg_scale":           4.0,
				"height":              1024,
				"width":               1024,
			},
		},
		Presets: []Preset{},
		Models: ModelConfig{
			Base:       []string{},
			LoRA:       []string{},
			ControlNet: []string{},
			VAE:        []string{},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=diffbox-config.json")
	json.NewEncoder(w).Encode(config)
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	var config UserConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid config format", http.StatusBadRequest)
		return
	}

	// TODO: Store config in database
	// TODO: Queue auto-downloads for pinned models

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "imported",
	})
}

func (s *Server) handleGetTokenStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: Check if tokens are configured (don't return actual values)
	status := TokenStatus{
		HuggingFace: false,
		Civitai:     false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleUpdateTokens(w http.ResponseWriter, r *http.Request) {
	var tokens TokenConfig
	if err := json.NewDecoder(r.Body).Decode(&tokens); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Store tokens securely

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":  "ok",
		"version": "0.1.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
