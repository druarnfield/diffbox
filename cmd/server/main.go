package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/druarnfield/diffbox/internal/api"
	"github.com/druarnfield/diffbox/internal/config"
	"github.com/druarnfield/diffbox/internal/db"
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

	// Start Python workers
	workerManager := worker.NewManager(cfg)
	if err := workerManager.Start(); err != nil {
		log.Fatalf("Failed to start workers: %v", err)
	}
	defer workerManager.Stop()

	// Create router
	router := api.NewRouter(cfg, database, q)

	// Create server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

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

	log.Printf("aria2 started with PID %d on port %s", cmd.Process.Pid, cfg.Aria2Port)
	return cmd, nil
}

func stopProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}
}
