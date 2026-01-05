# I2V + Qwen + CI/CD Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship working I2V and Qwen workflows with CI/CD pipeline for RunPod deployment.

**Architecture:** Go server spawns aria2 on startup, downloads missing models, then accepts jobs. Python workers load diffsynth pipelines and run real inference. GitHub Actions builds and pushes Docker images to GHCR.

**Tech Stack:** Go 1.23, Chi router, aria2 JSON-RPC, diffsynth (vendored), GitHub Actions, GHCR

---

## Task 1: CI/CD Pipeline - GitHub Actions Workflow

**Files:**
- Create: `.github/workflows/build.yml`

**Step 1: Create the workflow file**

```yaml
name: Build and Push

on:
  push:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'

      - name: Install uv
        uses: astral-sh/setup-uv@v4

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: |
          cd python && uv sync --frozen
          cd ../web && npm ci

      - name: Lint
        run: make lint

  test:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'

      - name: Install uv
        uses: astral-sh/setup-uv@v4

      - name: Install Python deps
        run: cd python && uv sync --frozen

      - name: Test
        run: make test

  build-and-push:
    runs-on: ubuntu-latest
    needs: test
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=latest
            type=sha,prefix=sha-

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

**Step 2: Commit**

```bash
git add .github/workflows/build.yml
git commit -m "feat: add CI/CD pipeline for GHCR"
```

---

## Task 2: aria2 Process Spawning

**Files:**
- Modify: `cmd/server/main.go`
- Create: `internal/aria2/client.go`

**Step 2.1: Create aria2 client package**

Create `internal/aria2/client.go`:

```go
package aria2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

type Client struct {
	url     string
	secret  string
	counter uint64
}

type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type DownloadStatus struct {
	GID             string `json:"gid"`
	Status          string `json:"status"`
	TotalLength     string `json:"totalLength"`
	CompletedLength string `json:"completedLength"`
	DownloadSpeed   string `json:"downloadSpeed"`
	ErrorCode       string `json:"errorCode,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
}

func NewClient(host string, port int, secret string) *Client {
	return &Client{
		url:    fmt.Sprintf("http://%s:%d/jsonrpc", host, port),
		secret: secret,
	}
}

func (c *Client) call(method string, params ...interface{}) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", atomic.AddUint64(&c.counter, 1))

	// Prepend token if secret is set
	if c.secret != "" {
		params = append([]interface{}{"token:" + c.secret}, params...)
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp Response
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// AddURI adds a download by URL, returns GID
func (c *Client) AddURI(url string, dir string, filename string, headers map[string]string) (string, error) {
	options := map[string]interface{}{
		"dir": dir,
		"out": filename,
	}

	if len(headers) > 0 {
		headerList := make([]string, 0, len(headers))
		for k, v := range headers {
			headerList = append(headerList, fmt.Sprintf("%s: %s", k, v))
		}
		options["header"] = headerList
	}

	result, err := c.call("aria2.addUri", []string{url}, options)
	if err != nil {
		return "", err
	}

	var gid string
	if err := json.Unmarshal(result, &gid); err != nil {
		return "", fmt.Errorf("unmarshal gid: %w", err)
	}

	return gid, nil
}

// TellStatus gets download status by GID
func (c *Client) TellStatus(gid string) (*DownloadStatus, error) {
	result, err := c.call("aria2.tellStatus", gid)
	if err != nil {
		return nil, err
	}

	var status DownloadStatus
	if err := json.Unmarshal(result, &status); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	return &status, nil
}

// TellActive gets all active downloads
func (c *Client) TellActive() ([]DownloadStatus, error) {
	result, err := c.call("aria2.tellActive")
	if err != nil {
		return nil, err
	}

	var statuses []DownloadStatus
	if err := json.Unmarshal(result, &statuses); err != nil {
		return nil, fmt.Errorf("unmarshal statuses: %w", err)
	}

	return statuses, nil
}

// Pause pauses a download
func (c *Client) Pause(gid string) error {
	_, err := c.call("aria2.pause", gid)
	return err
}

// Remove removes a download
func (c *Client) Remove(gid string) error {
	_, err := c.call("aria2.remove", gid)
	return err
}

// GetVersion checks aria2 is running
func (c *Client) GetVersion() (string, error) {
	result, err := c.call("aria2.getVersion")
	if err != nil {
		return "", err
	}

	var version struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(result, &version); err != nil {
		return "", fmt.Errorf("unmarshal version: %w", err)
	}

	return version.Version, nil
}
```

**Step 2.2: Implement startAria2 in main.go**

Modify `cmd/server/main.go`, replace the stub `startAria2` function:

```go
func startAria2(cfg *config.Config) (*exec.Cmd, error) {
	cmd := exec.Command("aria2c",
		"--enable-rpc",
		"--rpc-listen-all=false",
		fmt.Sprintf("--rpc-listen-port=%d", cfg.Aria2Port),
		"--rpc-allow-origin-all",
		fmt.Sprintf("--max-connection-per-server=%d", cfg.Aria2MaxConnections),
		"--split=16",
		"--min-split-size=1M",
		"--max-concurrent-downloads=4",
		"--continue=true",
		"--auto-file-renaming=false",
		"--allow-overwrite=true",
		fmt.Sprintf("--dir=%s", cfg.ModelsDir),
		"--daemon=false",
		"--quiet=true",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start aria2: %w", err)
	}

	log.Printf("aria2 started with PID %d on port %d", cmd.Process.Pid, cfg.Aria2Port)
	return cmd, nil
}
```

**Step 2.3: Implement startValkey in main.go**

Replace the stub `startValkey` function:

```go
func startValkey(cfg *config.Config) (*exec.Cmd, error) {
	cmd := exec.Command("valkey-server",
		"--port", fmt.Sprintf("%d", cfg.ValkeyPort),
		"--bind", "127.0.0.1",
		"--daemonize", "no",
		"--appendonly", "no",
		"--save", "",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start valkey: %w", err)
	}

	log.Printf("Valkey started with PID %d on port %d", cmd.Process.Pid, cfg.ValkeyPort)
	return cmd, nil
}
```

**Step 2.4: Commit**

```bash
git add internal/aria2/client.go cmd/server/main.go
git commit -m "feat: implement aria2 and valkey process spawning"
```

---

## Task 3: Model Auto-Download System

**Files:**
- Create: `internal/models/download.go`
- Modify: `cmd/server/main.go`

**Step 3.1: Create model download manager**

Create `internal/models/download.go`:

```go
package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"diffbox/internal/aria2"
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
```

**Step 3.2: Integrate download check into startup**

Modify `cmd/server/main.go` to add download check after aria2 starts. Add this after the aria2 startup and before starting workers:

```go
// After aria2 starts, add:
import "diffbox/internal/aria2"
import "diffbox/internal/models"

// In main(), after aria2 start:
aria2Client := aria2.NewClient("localhost", cfg.Aria2Port, "")

// Wait for aria2 to be ready
for i := 0; i < 10; i++ {
	if _, err := aria2Client.GetVersion(); err == nil {
		break
	}
	time.Sleep(500 * time.Millisecond)
}

// Download missing models
hfToken := os.Getenv("HF_TOKEN")
downloader := models.NewDownloader(aria2Client, cfg.ModelsDir, hfToken)
if err := downloader.CheckAndDownload(); err != nil {
	log.Fatalf("Model download failed: %v", err)
}
```

**Step 3.3: Commit**

```bash
git add internal/models/download.go cmd/server/main.go
git commit -m "feat: add model auto-download on startup"
```

---

## Task 4: I2V Python Worker - Real Inference

**Files:**
- Modify: `python/worker/i2v.py`

**Step 4.1: Implement real I2V pipeline loading and inference**

Replace `python/worker/i2v.py`:

```python
"""Wan 2.2 I2V (Image-to-Video) worker handler."""

import os
import torch
from pathlib import Path
from PIL import Image
import base64
from io import BytesIO

from .protocol import send_progress

# Import diffsynth components
from diffsynth.pipelines.wan_video import WanVideoPipeline
from diffsynth.core import ModelConfig


class I2VHandler:
    """Handler for Wan 2.2 Image-to-Video generation."""

    def __init__(self, models_dir: str, outputs_dir: str):
        self.models_dir = Path(models_dir)
        self.outputs_dir = Path(outputs_dir)
        self.pipeline = None

    def _load_pipeline(self):
        """Load the Wan 2.2 I2V pipeline."""
        if self.pipeline is not None:
            return

        send_progress(None, 0.0, "Loading I2V pipeline...")

        # Model paths from Comfy-Org repackaged
        high_noise_path = self.models_dir / "wan2.2_i2v_high_noise_14B_fp16.safetensors"
        low_noise_path = self.models_dir / "wan2.2_i2v_low_noise_14B_fp16.safetensors"
        text_encoder_path = self.models_dir / "umt5_xxl_fp16.safetensors"
        vae_path = self.models_dir / "wan_2.1_vae.safetensors"

        # Configure models
        model_configs = [
            # High noise DiT (first expert)
            ModelConfig(path=str(high_noise_path)),
            # Low noise DiT (second expert)
            ModelConfig(path=str(low_noise_path)),
            # Text encoder
            ModelConfig(path=str(text_encoder_path)),
            # VAE
            ModelConfig(path=str(vae_path)),
        ]

        self.pipeline = WanVideoPipeline.from_pretrained(
            torch_dtype=torch.bfloat16,
            device="cuda",
            model_configs=model_configs,
        )

        send_progress(None, 0.0, "Pipeline loaded")

    def run(self, job_id: str, params: dict) -> dict:
        """Run I2V inference."""
        self._load_pipeline()

        # Extract parameters with defaults
        prompt = params.get("prompt", "")
        negative_prompt = params.get("negative_prompt", "")
        input_image_b64 = params.get("input_image", "")
        seed = params.get("seed", -1)
        height = params.get("height", 480)
        width = params.get("width", 832)
        num_frames = params.get("num_frames", 81)
        num_inference_steps = params.get("num_inference_steps", 50)
        cfg_scale = params.get("cfg_scale", 5.0)
        denoising_strength = params.get("denoising_strength", 1.0)

        # Decode input image
        if input_image_b64:
            image_data = base64.b64decode(input_image_b64)
            input_image = Image.open(BytesIO(image_data)).convert("RGB")
            input_image = input_image.resize((width, height))
        else:
            raise ValueError("input_image is required for I2V")

        # Set seed
        if seed == -1:
            seed = torch.randint(0, 2**32, (1,)).item()
        generator = torch.Generator(device="cuda").manual_seed(seed)

        send_progress(job_id, 0.05, "Starting inference")

        # Progress callback
        def progress_callback(step, total_steps, latents):
            progress = 0.05 + (step / total_steps) * 0.90
            send_progress(job_id, progress, f"Step {step}/{total_steps}")

        # Run inference
        video_frames = self.pipeline(
            prompt=prompt,
            negative_prompt=negative_prompt,
            input_image=input_image,
            height=height,
            width=width,
            num_frames=num_frames,
            num_inference_steps=num_inference_steps,
            guidance_scale=cfg_scale,
            denoising_strength=denoising_strength,
            generator=generator,
            callback=progress_callback,
            callback_steps=1,
        )

        send_progress(job_id, 0.95, "Encoding video")

        # Save video
        output_path = self.outputs_dir / f"{job_id}.mp4"
        self._save_video(video_frames, output_path)

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "video",
            "path": str(output_path),
            "frames": num_frames,
            "seed": seed,
        }

    def _save_video(self, frames: list, output_path: Path, fps: int = 24):
        """Save frames as MP4 video."""
        import subprocess
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            # Save frames as images
            for i, frame in enumerate(frames):
                if isinstance(frame, torch.Tensor):
                    frame = frame.cpu().numpy()
                    frame = (frame * 255).astype("uint8")
                    frame = Image.fromarray(frame)
                frame.save(f"{tmpdir}/frame_{i:05d}.png")

            # Encode with ffmpeg
            subprocess.run([
                "ffmpeg", "-y",
                "-framerate", str(fps),
                "-i", f"{tmpdir}/frame_%05d.png",
                "-c:v", "libx264",
                "-pix_fmt", "yuv420p",
                "-crf", "18",
                str(output_path)
            ], check=True, capture_output=True)
```

**Step 4.2: Commit**

```bash
git add python/worker/i2v.py
git commit -m "feat: implement real I2V inference with diffsynth"
```

---

## Task 5: Qwen Python Worker - Real Inference

**Files:**
- Modify: `python/worker/qwen.py`

**Step 5.1: Implement real Qwen pipeline**

Replace `python/worker/qwen.py`:

```python
"""Qwen Image Edit 2511 worker handler."""

import os
import torch
from pathlib import Path
from PIL import Image
import base64
from io import BytesIO

from .protocol import send_progress

# Import diffsynth components
from diffsynth.pipelines.qwen_image import QwenImagePipeline
from diffsynth.core import ModelConfig


class QwenHandler:
    """Handler for Qwen Image Edit 2511."""

    def __init__(self, models_dir: str, outputs_dir: str):
        self.models_dir = Path(models_dir)
        self.outputs_dir = Path(outputs_dir)
        self.pipeline = None

    def _load_pipeline(self):
        """Load the Qwen Image Edit pipeline."""
        if self.pipeline is not None:
            return

        send_progress(None, 0.0, "Loading Qwen pipeline...")

        # Model paths from Comfy-Org repackaged
        dit_path = self.models_dir / "qwen_image_edit_2511_bf16.safetensors"
        text_encoder_path = self.models_dir / "qwen_2.5_vl_7b.safetensors"
        vae_path = self.models_dir / "qwen_image_vae.safetensors"

        model_configs = [
            ModelConfig(path=str(dit_path)),
            ModelConfig(path=str(text_encoder_path)),
            ModelConfig(path=str(vae_path)),
        ]

        self.pipeline = QwenImagePipeline.from_pretrained(
            torch_dtype=torch.bfloat16,
            device="cuda",
            model_configs=model_configs,
        )

        send_progress(None, 0.0, "Pipeline loaded")

    def run(self, job_id: str, params: dict) -> dict:
        """Run Qwen image edit inference."""
        self._load_pipeline()

        # Extract parameters
        instruction = params.get("instruction", "")
        edit_images_b64 = params.get("edit_images", [])
        height = params.get("height", 1024)
        width = params.get("width", 1024)
        num_inference_steps = params.get("num_inference_steps", 30)
        cfg_scale = params.get("cfg_scale", 4.0)
        seed = params.get("seed", -1)

        # Decode input images (up to 3)
        edit_images = []
        for img_b64 in edit_images_b64[:3]:
            if img_b64:
                image_data = base64.b64decode(img_b64)
                img = Image.open(BytesIO(image_data)).convert("RGB")
                edit_images.append(img)

        if not edit_images:
            raise ValueError("At least one edit_image is required")

        # Set seed
        if seed == -1:
            seed = torch.randint(0, 2**32, (1,)).item()
        generator = torch.Generator(device="cuda").manual_seed(seed)

        send_progress(job_id, 0.05, "Starting inference")

        # Progress callback
        def progress_callback(step, total_steps, latents):
            progress = 0.05 + (step / total_steps) * 0.90
            send_progress(job_id, progress, f"Step {step}/{total_steps}")

        # Run inference
        output_image = self.pipeline(
            prompt=instruction,
            input_images=edit_images,
            height=height,
            width=width,
            num_inference_steps=num_inference_steps,
            guidance_scale=cfg_scale,
            generator=generator,
            callback=progress_callback,
            callback_steps=1,
        )

        send_progress(job_id, 0.95, "Saving image")

        # Save output
        output_path = self.outputs_dir / f"{job_id}.png"

        if isinstance(output_image, torch.Tensor):
            output_image = output_image.cpu().numpy()
            output_image = (output_image * 255).astype("uint8")
            output_image = Image.fromarray(output_image)

        output_image.save(output_path)

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "image",
            "path": str(output_path),
            "seed": seed,
        }
```

**Step 5.2: Commit**

```bash
git add python/worker/qwen.py
git commit -m "feat: implement real Qwen image edit inference"
```

---

## Task 6: WebSocket Progress Broadcasting

**Files:**
- Modify: `internal/worker/manager.go`
- Modify: `internal/api/websocket.go`

**Step 6.1: Add WebSocket hub reference to worker manager**

Modify `internal/worker/manager.go` to broadcast progress:

```go
// Add to Manager struct:
type Manager struct {
	workers []*Worker
	config  *config.Config
	mu      sync.Mutex
	db      *db.DB
	wsHub   *WebSocketHub  // Add this
}

// Update NewManager:
func NewManager(cfg *config.Config, database *db.DB, wsHub *WebSocketHub) *Manager {
	return &Manager{
		config: cfg,
		db:     database,
		wsHub:  wsHub,
	}
}

// Update handleWorkerOutput to broadcast:
func (m *Manager) handleWorkerOutput(w *Worker) {
	scanner := bufio.NewScanner(w.stdout)
	for scanner.Scan() {
		line := scanner.Text()

		var msg WorkerMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Worker %s invalid JSON: %s", w.id, line)
			continue
		}

		switch msg.Type {
		case "progress":
			var progress ProgressUpdate
			if err := json.Unmarshal(msg.Data, &progress); err == nil {
				// Update DB
				m.db.UpdateJobProgress(progress.JobID, progress.Progress, progress.Stage)
				// Broadcast via WebSocket
				m.wsHub.BroadcastJobProgress(progress.JobID, progress.Progress, progress.Stage, progress.Preview)
			}

		case "complete":
			var result JobResult
			if err := json.Unmarshal(msg.Data, &result); err == nil {
				outputJSON, _ := json.Marshal(result.Output)
				m.db.CompleteJob(result.JobID, string(outputJSON))
				m.wsHub.BroadcastJobComplete(result.JobID, result.Output)
			}

		case "error":
			var result JobResult
			if err := json.Unmarshal(msg.Data, &result); err == nil {
				m.db.FailJob(result.JobID, result.Error)
				m.wsHub.BroadcastJobError(result.JobID, result.Error)
			}

		case "ready":
			log.Printf("Worker %s ready", w.id)
		}
	}
}
```

**Step 6.2: Add broadcast methods to WebSocket hub**

Add to `internal/api/websocket.go`:

```go
// BroadcastJobProgress sends progress update to all clients
func (h *Hub) BroadcastJobProgress(jobID string, progress float64, stage string, preview string) {
	msg := map[string]interface{}{
		"type": "job:progress",
		"data": JobProgress{
			JobID:    jobID,
			Progress: progress,
			Stage:    stage,
			Preview:  preview,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

// BroadcastJobComplete sends completion to all clients
func (h *Hub) BroadcastJobComplete(jobID string, output interface{}) {
	msg := map[string]interface{}{
		"type": "job:complete",
		"data": JobComplete{
			JobID:  jobID,
			Output: output,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}

// BroadcastJobError sends error to all clients
func (h *Hub) BroadcastJobError(jobID string, errorMsg string) {
	msg := map[string]interface{}{
		"type": "job:error",
		"data": JobError{
			JobID: jobID,
			Error: errorMsg,
		},
	}
	data, _ := json.Marshal(msg)
	h.broadcast <- data
}
```

**Step 6.3: Commit**

```bash
git add internal/worker/manager.go internal/api/websocket.go
git commit -m "feat: broadcast worker progress via WebSocket"
```

---

## Task 7: I2V Frontend Form

**Files:**
- Modify: `web/src/pages/WorkflowPage.tsx`

**Step 7.1: Implement I2V form with API integration**

> **Note:** Use `frontend-design` skill and shadcn MCP for this task.

The form needs:
- Image upload with drag & drop
- Prompt textarea
- Generate button
- Progress bar connected to WebSocket
- Video output player

Key implementation points:
- Use `useMutation` from TanStack Query for form submission
- Use custom `useWebSocket` hook for progress updates
- Store uploaded image as base64
- Show progress percentage from WebSocket

**Step 7.2: Commit**

```bash
git add web/src/pages/WorkflowPage.tsx
git commit -m "feat: implement I2V form UI with progress"
```

---

## Task 8: Qwen Frontend Form

**Files:**
- Create: `web/src/components/QwenForm.tsx`
- Modify: `web/src/pages/WorkflowPage.tsx`

**Step 8.1: Create Qwen form component**

> **Note:** Use `frontend-design` skill and shadcn MCP for this task.

The form needs:
- 3 image upload slots (first required, 2-3 optional)
- Instruction textarea
- Generate button
- Progress bar
- Image output display

**Step 8.2: Commit**

```bash
git add web/src/components/QwenForm.tsx web/src/pages/WorkflowPage.tsx
git commit -m "feat: implement Qwen form UI with 3 image uploads"
```

---

## Task 9: Output Gallery

**Files:**
- Create: `web/src/components/OutputGallery.tsx`
- Modify: `web/src/pages/WorkflowPage.tsx`

**Step 9.1: Create output gallery component**

> **Note:** Use `frontend-design` skill and shadcn MCP for this task.

Features:
- Display completed jobs with thumbnails
- Video player for I2V outputs
- Image viewer for Qwen outputs
- Download button for each output
- Job metadata (seed, timestamps)

**Step 9.2: Commit**

```bash
git add web/src/components/OutputGallery.tsx web/src/pages/WorkflowPage.tsx
git commit -m "feat: add output gallery with download support"
```

---

## Task 10: Integration Testing

**Files:**
- Create: `tests/integration/workflow_test.go`

**Step 10.1: Write integration test**

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestI2VWorkflowSubmission(t *testing.T) {
	// Skip in CI (no GPU)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that job submission works
	payload := map[string]interface{}{
		"prompt":      "A cat walking",
		"input_image": "base64encodedimage...",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/workflows/i2v", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// ... test implementation
}
```

**Step 10.2: Commit**

```bash
git add tests/integration/workflow_test.go
git commit -m "test: add workflow integration tests"
```

---

## Task 11: Final Verification

**Step 11.1: Run full test suite**

```bash
make lint
make test
make build
make frontend
```

**Step 11.2: Test Docker build locally**

```bash
make docker
```

**Step 11.3: Push to trigger CI/CD**

```bash
git push origin feature/i2v-qwen-cicd
```

Verify in GitHub Actions that:
- Lint passes
- Tests pass
- Docker image builds
- Image pushed to GHCR

**Step 11.4: Create PR**

```bash
gh pr create --title "feat: I2V + Qwen workflows with CI/CD" --body "## Summary
- GitHub Actions CI/CD pipeline to GHCR
- Model auto-download via aria2 (~127GB)
- I2V workflow (basic tier)
- Qwen workflow (basic tier, 3 images)

## Test Plan
- [ ] CI/CD builds and pushes to GHCR
- [ ] RunPod pulls image successfully
- [ ] Models download on first startup
- [ ] I2V generates video from image+prompt
- [ ] Qwen generates edited image from 3 images+instruction
- [ ] Progress updates appear in UI"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | CI/CD Pipeline | `.github/workflows/build.yml` |
| 2 | aria2 Process Spawning | `internal/aria2/client.go`, `cmd/server/main.go` |
| 3 | Model Auto-Download | `internal/models/download.go` |
| 4 | I2V Python Worker | `python/worker/i2v.py` |
| 5 | Qwen Python Worker | `python/worker/qwen.py` |
| 6 | WebSocket Broadcasting | `internal/worker/manager.go`, `internal/api/websocket.go` |
| 7 | I2V Frontend Form | `web/src/pages/WorkflowPage.tsx` |
| 8 | Qwen Frontend Form | `web/src/components/QwenForm.tsx` |
| 9 | Output Gallery | `web/src/components/OutputGallery.tsx` |
| 10 | Integration Tests | `tests/integration/workflow_test.go` |
| 11 | Final Verification | PR creation |

**Total: 11 tasks, ~15 commits**
