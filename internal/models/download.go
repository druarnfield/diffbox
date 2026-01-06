package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/druarnfield/diffbox/internal/aria2"
)

// ModelFile represents a required model file
type ModelFile struct {
	Name     string // Local filename
	URL      string // HuggingFace URL
	Size     int64  // Expected size in bytes
	Workflow string // Which workflow needs this
}

// RequiredModels returns all models needed for I2V and Qwen workflows
func RequiredModels() []ModelFile {
	hfBase := "https://huggingface.co"

	return []ModelFile{
		// Wan 2.2 I2V - High Noise DiT
		{
			Name:     "wan2.2_i2v_high_noise_14B_fp16.safetensors",
			URL:      hfBase + "/Comfy-Org/Wan_2.2_ComfyUI_Repackaged/resolve/main/split_files/diffusion_models/wan2.2_i2v_high_noise_14B_fp16.safetensors",
			Size:     28_600_000_000,
			Workflow: "i2v",
		},
		// Wan 2.2 I2V - Low Noise DiT
		{
			Name:     "wan2.2_i2v_low_noise_14B_fp16.safetensors",
			URL:      hfBase + "/Comfy-Org/Wan_2.2_ComfyUI_Repackaged/resolve/main/split_files/diffusion_models/wan2.2_i2v_low_noise_14B_fp16.safetensors",
			Size:     28_600_000_000,
			Workflow: "i2v",
		},
		// Wan 2.2 - T5 Text Encoder
		{
			Name:     "umt5_xxl_fp16.safetensors",
			URL:      hfBase + "/Comfy-Org/Wan_2.2_ComfyUI_Repackaged/resolve/main/split_files/text_encoders/umt5_xxl_fp16.safetensors",
			Size:     11_400_000_000,
			Workflow: "i2v",
		},
		// Wan 2.2 - VAE
		{
			Name:     "wan_2.1_vae.safetensors",
			URL:      hfBase + "/Comfy-Org/Wan_2.2_ComfyUI_Repackaged/resolve/main/split_files/vae/wan_2.1_vae.safetensors",
			Size:     254_000_000,
			Workflow: "i2v",
		},
		// Wan 2.2 - Lightning LoRA High Noise (4-step distilled model)
		{
			Name:     "wan2.2_lightning_high_noise.safetensors",
			URL:      hfBase + "/lightx2v/Wan2.2-Lightning/resolve/main/Wan2.2-I2V-A14B-4steps-lora-rank64-Seko-V1/high_noise_model.safetensors",
			Size:     1_230_000_000,
			Workflow: "i2v",
		},
		// Wan 2.2 - Lightning LoRA Low Noise (4-step distilled model)
		{
			Name:     "wan2.2_lightning_low_noise.safetensors",
			URL:      hfBase + "/lightx2v/Wan2.2-Lightning/resolve/main/Wan2.2-I2V-A14B-4steps-lora-rank64-Seko-V1/low_noise_model.safetensors",
			Size:     1_230_000_000,
			Workflow: "i2v",
		},
		// Qwen Image Edit 2511 - DiT
		{
			Name:     "qwen_image_edit_2511_bf16.safetensors",
			URL:      hfBase + "/Comfy-Org/Qwen-Image-Edit_ComfyUI/resolve/main/split_files/diffusion_models/qwen_image_edit_2511_bf16.safetensors",
			Size:     40_900_000_000,
			Workflow: "qwen",
		},
		// Qwen - Text Encoder
		{
			Name:     "qwen_2.5_vl_7b.safetensors",
			URL:      hfBase + "/Comfy-Org/Qwen-Image_ComfyUI/resolve/main/split_files/text_encoders/qwen_2.5_vl_7b.safetensors",
			Size:     16_600_000_000,
			Workflow: "qwen",
		},
		// Qwen - VAE
		{
			Name:     "qwen_image_vae.safetensors",
			URL:      hfBase + "/Comfy-Org/Qwen-Image_ComfyUI/resolve/main/split_files/vae/qwen_image_vae.safetensors",
			Size:     254_000_000,
			Workflow: "qwen",
		},
		// Qwen - Lightning LoRA (4-step distilled model)
		{
			Name:     "Qwen-Image-Edit-2511-Lightning-4steps-V1.0-bf16.safetensors",
			URL:      hfBase + "/lightx2v/Qwen-Image-Edit-2511-Lightning/resolve/main/Qwen-Image-Edit-2511-Lightning-4steps-V1.0-bf16.safetensors",
			Size:     200_000_000, // ~200MB LoRA weights
			Workflow: "qwen",
		},
		// Qwen - Tokenizer files (for Qwen2Tokenizer)
		{
			Name:     "qwen_tokenizer/tokenizer.json",
			URL:      hfBase + "/Qwen/Qwen2.5-VL-7B-Instruct/resolve/main/tokenizer.json",
			Size:     7_030_000, // ~7MB
			Workflow: "qwen",
		},
		{
			Name:     "qwen_tokenizer/vocab.json",
			URL:      hfBase + "/Qwen/Qwen2.5-VL-7B-Instruct/resolve/main/vocab.json",
			Size:     2_780_000, // ~2.8MB
			Workflow: "qwen",
		},
		{
			Name:     "qwen_tokenizer/merges.txt",
			URL:      hfBase + "/Qwen/Qwen2.5-VL-7B-Instruct/resolve/main/merges.txt",
			Size:     1_670_000, // ~1.7MB
			Workflow: "qwen",
		},
		{
			Name:     "qwen_tokenizer/tokenizer_config.json",
			URL:      hfBase + "/Qwen/Qwen2.5-VL-7B-Instruct/resolve/main/tokenizer_config.json",
			Size:     6_000, // ~6KB
			Workflow: "qwen",
		},
	}
}

// Downloader manages model downloads via aria2
type Downloader struct {
	client    *aria2.Client
	modelsDir string
	hfToken   string
}

// NewDownloader creates a new downloader
func NewDownloader(client *aria2.Client, modelsDir, hfToken string) *Downloader {
	return &Downloader{
		client:    client,
		modelsDir: modelsDir,
		hfToken:   hfToken,
	}
}

// CheckAndDownload checks for missing models and downloads them
func (d *Downloader) CheckAndDownload() error {
	required := RequiredModels()
	missing := d.findMissing(required)

	if len(missing) == 0 {
		log.Println("All required models present")
		return nil
	}

	log.Printf("Downloading %d missing models...", len(missing))

	// Queue all downloads
	gids := make(map[string]ModelFile)
	for _, model := range missing {
		headers := map[string]string{}
		if d.hfToken != "" {
			headers["Authorization"] = "Bearer " + d.hfToken
		}

		gid, err := d.client.AddURI(model.URL, d.modelsDir, model.Name, headers)
		if err != nil {
			return fmt.Errorf("queue download %s: %w", model.Name, err)
		}
		gids[gid] = model
		log.Printf("Queued: %s", model.Name)
	}

	// Wait for all downloads to complete
	return d.waitForDownloads(gids)
}

func (d *Downloader) findMissing(models []ModelFile) []ModelFile {
	var missing []ModelFile

	for _, model := range models {
		path := filepath.Join(d.modelsDir, model.Name)
		info, err := os.Stat(path)

		if os.IsNotExist(err) {
			missing = append(missing, model)
			continue
		}

		if err != nil {
			// Permission or other error - treat as missing
			log.Printf("Cannot stat %s: %v (will download)", model.Name, err)
			missing = append(missing, model)
			continue
		}

		// Check size (allow 1% tolerance for filesystem differences)
		if info.Size() < int64(float64(model.Size)*0.99) {
			log.Printf("Incomplete: %s (%.2f GB / %.2f GB)",
				model.Name,
				float64(info.Size())/1e9,
				float64(model.Size)/1e9)
			missing = append(missing, model)
		}
	}

	return missing
}

func (d *Downloader) waitForDownloads(gids map[string]ModelFile) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for len(gids) > 0 {
		<-ticker.C

		for gid, model := range gids {
			status, err := d.client.TellStatus(gid)
			if err != nil {
				log.Printf("Status check failed for %s: %v", model.Name, err)
				continue
			}

			switch status.Status {
			case "complete":
				log.Printf("Complete: %s", model.Name)
				delete(gids, gid)

			case "error":
				return fmt.Errorf("download failed %s: %s", model.Name, status.ErrorMessage)

			case "active":
				// Parse progress
				total := parseSize(status.TotalLength)
				completed := parseSize(status.CompletedLength)
				speed := parseSize(status.DownloadSpeed)

				if total > 0 {
					pct := float64(completed) / float64(total) * 100
					log.Printf("Downloading %s: %.1f%% (%.2f MB/s)",
						model.Name, pct, float64(speed)/1e6)
				}

			case "waiting":
				log.Printf("Waiting: %s (queued)", model.Name)

			case "paused":
				log.Printf("Paused: %s (resuming...)", model.Name)
			}
		}
	}

	return nil
}

func parseSize(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
