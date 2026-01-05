package main

import (
	"context"
	"log"
	"net/http"
	"os"
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

func startValkey(cfg *config.Config) (*os.Process, error) {
	// TODO: Implement Valkey process spawning
	log.Println("Starting Valkey...")
	return nil, nil
}

func startAria2(cfg *config.Config) (*os.Process, error) {
	// TODO: Implement aria2 process spawning
	log.Println("Starting aria2...")
	return nil, nil
}

func stopProcess(p *os.Process) {
	if p != nil {
		p.Signal(syscall.SIGTERM)
		p.Wait()
	}
}
