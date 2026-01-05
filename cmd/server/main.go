package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/druarnfield/diffbox/internal/api"
	"github.com/druarnfield/diffbox/internal/aria2"
	"github.com/druarnfield/diffbox/internal/config"
	"github.com/druarnfield/diffbox/internal/db"
	"github.com/druarnfield/diffbox/internal/models"
	"github.com/druarnfield/diffbox/internal/queue"
	"github.com/druarnfield/diffbox/internal/worker"
)

func main() {
	log.Println("Starting diffbox...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.New(cfg.DataDir + "/diffbox.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Start Valkey (Redis)
	valkeyProcess, err := startValkey(cfg)
	if err != nil {
		log.Fatalf("Failed to start Valkey: %v", err)
	}
	defer stopProcess(valkeyProcess)

	// Wait for Valkey to be ready
	time.Sleep(1 * time.Second)

	// Initialize queue
	q, err := queue.NewRedisQueue(cfg.ValkeyAddr)
	if err != nil {
		log.Fatalf("Failed to initialize queue: %v", err)
	}
	defer q.Close()

	// Start aria2 daemon
	aria2Process, err := startAria2(cfg)
	if err != nil {
		log.Fatalf("Failed to start aria2: %v", err)
	}
	defer stopProcess(aria2Process)

	// Create aria2 client and wait for it to be ready
	aria2Port, err := strconv.Atoi(cfg.Aria2Port)
	if err != nil {
		log.Fatalf("Invalid aria2 port: %v", err)
	}
	// Use 127.0.0.1 instead of localhost to avoid IPv6 resolution issues
	aria2Client := aria2.NewClient("127.0.0.1", aria2Port, "")

	// Give aria2 a moment to initialize before first connection attempt
	time.Sleep(1 * time.Second)

	// Wait for aria2 to be ready
	aria2Ready := false
	var lastErr error
	for i := 0; i < 10; i++ {
		// Check if process is still running
		if aria2Process.ProcessState != nil {
			log.Fatalf("aria2 process exited prematurely with state: %v", aria2Process.ProcessState)
		}

		version, err := aria2Client.GetVersion()
		if err == nil {
			log.Printf("aria2 is ready (version: %s)", version)
			aria2Ready = true
			break
		}
		lastErr = err
		log.Printf("Waiting for aria2 to be ready (attempt %d/10): %v", i+1, err)
		time.Sleep(500 * time.Millisecond)
	}
	if !aria2Ready {
		log.Fatalf("aria2 failed to become ready after 10 attempts. Last error: %v", lastErr)
	}

	// Create router (start webserver early so user can see progress)
	router, wsHub := api.NewRouter(cfg, database, q, aria2Client)

	// Create server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in background
	go func() {
		log.Printf("Server listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Download missing models in background (non-blocking)
	go func() {
		log.Println("Starting model download check...")
		hfToken := os.Getenv("HF_TOKEN")
		downloader := models.NewDownloader(aria2Client, cfg.ModelsDir, hfToken)
		if err := downloader.CheckAndDownload(); err != nil {
			log.Printf("Model download failed: %v", err)
			log.Println("Server will continue running, but workflows may fail without models")
		} else {
			log.Println("All models ready!")
		}
	}()

	// Start Python workers (they'll wait for models when processing jobs)
	workerManager := worker.NewManager(cfg)
	if err := workerManager.Start(); err != nil {
		log.Fatalf("Failed to start workers: %v", err)
	}
	defer workerManager.Stop()

	// Wire up worker callbacks to WebSocket hub
	workerManager.SetCallbacks(
		// Progress callback
		func(progress worker.ProgressUpdate) {
			wsHub.BroadcastJobProgress(api.JobProgress{
				JobID:    progress.JobID,
				Progress: progress.Progress,
				Stage:    progress.Stage,
				Preview:  progress.Preview,
			})
		},
		// Complete callback
		func(result worker.JobResult) {
			wsHub.BroadcastJobComplete(api.JobComplete{
				JobID: result.JobID,
				Output: api.JobOutput{
					Type: "output",
					Path: result.Output,
				},
			})
		},
		// Error callback
		func(result worker.JobResult) {
			wsHub.BroadcastJobError(api.JobError{
				JobID: result.JobID,
				Error: result.Error,
			})
		},
	)

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-done
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Goodbye!")
}

func startValkey(cfg *config.Config) (*exec.Cmd, error) {
	cmd := exec.Command("valkey-server",
		"--port", cfg.ValkeyPort,
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

	log.Printf("Valkey started with PID %d on port %s", cmd.Process.Pid, cfg.ValkeyPort)
	return cmd, nil
}

func startAria2(cfg *config.Config) (*exec.Cmd, error) {
	cmd := exec.Command("aria2c",
		"--enable-rpc",
		"--rpc-listen-all=false",
		fmt.Sprintf("--rpc-listen-port=%s", cfg.Aria2Port),
		"--rpc-allow-origin-all",
		"--disable-ipv6",
		fmt.Sprintf("--max-connection-per-server=%d", cfg.Aria2MaxConnections),
		"--split=16",
		"--min-split-size=1M",
		"--max-concurrent-downloads=4",
		"--continue=true",
		"--auto-file-renaming=false",
		"--allow-overwrite=true",
		fmt.Sprintf("--dir=%s", cfg.ModelsDir),
		"--daemon=false",
		"--console-log-level=notice",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start aria2: %w", err)
	}

	log.Printf("aria2 started with PID %d on port %s", cmd.Process.Pid, cfg.Aria2Port)
	return cmd, nil
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
	}
}
