package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/druarnfield/diffbox/internal/config"
)

// ProgressCallback is called when a worker reports progress
type ProgressCallback func(ProgressUpdate)

// CompleteCallback is called when a worker completes a job
type CompleteCallback func(JobResult)

// ErrorCallback is called when a worker reports an error
type ErrorCallback func(JobResult)

type Manager struct {
	cfg        *config.Config
	workers    []*Worker
	nextWorker int
	mu         sync.Mutex
	onProgress ProgressCallback
	onComplete CompleteCallback
	onError    ErrorCallback
}

type Worker struct {
	id      int
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	running bool
}

type WorkerMessage struct {
	Type  string          `json:"type"`
	JobID string          `json:"job_id,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type JobRequest struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

type ProgressUpdate struct {
	JobID    string  `json:"job_id"`
	Progress float64 `json:"progress"`
	Stage    string  `json:"stage"`
	Preview  string  `json:"preview,omitempty"`
}

type JobResult struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:     cfg,
		workers: make([]*Worker, 0),
	}
}

// SetCallbacks sets the callback functions for worker events
func (m *Manager) SetCallbacks(onProgress ProgressCallback, onComplete CompleteCallback, onError ErrorCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onProgress = onProgress
	m.onComplete = onComplete
	m.onError = onError
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 0; i < m.cfg.WorkerCount; i++ {
		worker, err := m.spawnWorker(i)
		if err != nil {
			return fmt.Errorf("failed to spawn worker %d: %w", i, err)
		}
		m.workers = append(m.workers, worker)
	}

	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, worker := range m.workers {
		if worker.running {
			// Send shutdown message
			msg := WorkerMessage{Type: "shutdown"}
			json.NewEncoder(worker.stdin).Encode(msg)
			worker.cmd.Wait()
			worker.running = false
		}
	}
}

func (m *Manager) spawnWorker(id int) (*Worker, error) {
	// Use uv to run the Python worker
	cmd := exec.Command("uv", "run", "python", "-m", "worker")
	cmd.Dir = m.cfg.PythonPath
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DIFFBOX_MODELS_DIR=%s", m.cfg.ModelsDir),
		fmt.Sprintf("DIFFBOX_OUTPUTS_DIR=%s", m.cfg.OutputsDir),
		fmt.Sprintf("WORKER_ID=%d", id),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	worker := &Worker{
		id:      id,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		running: false,
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	worker.running = true

	// Handle stdout (JSON messages)
	go m.handleWorkerOutput(worker)

	// Handle stderr (logs)
	go m.handleWorkerLogs(worker)

	// Monitor worker process health
	go func() {
		err := cmd.Wait()
		worker.running = false
		if err != nil {
			log.Printf("ERROR - Worker %d exited with error: %v", id, err)
		} else {
			log.Printf("Worker %d exited cleanly", id)
		}
	}()

	log.Printf("Worker %d started (PID: %d)", id, cmd.Process.Pid)

	return worker, nil
}

func (m *Manager) handleWorkerOutput(w *Worker) {
	scanner := bufio.NewScanner(w.stdout)

	// Increase buffer size for large JSON messages (default is 64KB)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()

		var msg WorkerMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Worker %d: invalid JSON: %s", w.id, line)
			continue
		}

		switch msg.Type {
		case "progress":
			var progress ProgressUpdate
			if err := json.Unmarshal(msg.Data, &progress); err != nil {
				log.Printf("Worker %d: invalid progress data: %v", w.id, err)
				continue
			}
			log.Printf("Worker %d: job %s progress %.1f%% - %s", w.id, progress.JobID, progress.Progress*100, progress.Stage)
			if m.onProgress != nil {
				m.onProgress(progress)
			}

		case "complete":
			var result JobResult
			if err := json.Unmarshal(msg.Data, &result); err != nil {
				log.Printf("Worker %d: invalid result data: %v", w.id, err)
				continue
			}
			log.Printf("Worker %d: job %s completed: %s", w.id, result.JobID, result.Output)
			if m.onComplete != nil {
				m.onComplete(result)
			}

		case "error":
			var result JobResult
			if err := json.Unmarshal(msg.Data, &result); err != nil {
				log.Printf("Worker %d: invalid error data: %v", w.id, err)
				continue
			}
			log.Printf("ERROR - Worker %d: job %s FAILED: %s", w.id, result.JobID, result.Error)
			if m.onError != nil {
				m.onError(result)
			}

		case "ready":
			log.Printf("Worker %d: ready", w.id)
		}
	}
}

func (m *Manager) handleWorkerLogs(w *Worker) {
	scanner := bufio.NewScanner(w.stderr)

	// Increase buffer size to handle long Python tracebacks/logs (default is 64KB)
	// Set to 1MB to handle very long error messages
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()

		// Only filter out very specific noisy warnings, not all site-packages
		if strings.Contains(line, "pynvml package is deprecated") {
			continue
		}

		// Log with worker ID prefix
		log.Printf("Worker %d: %s", w.id, line)
	}

	// Log when stderr closes (worker exited)
	if err := scanner.Err(); err != nil {
		log.Printf("ERROR - Worker %d stderr closed with error: %v", w.id, err)
	} else {
		log.Printf("Worker %d stderr closed (worker may have exited)", w.id)
	}
}

func (m *Manager) SubmitJob(job *JobRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find an available worker using round-robin scheduling
	if len(m.workers) == 0 {
		log.Printf("ERROR - Cannot submit job %s: no workers available", job.ID)
		return fmt.Errorf("no workers available")
	}

	var worker *Worker
	for i := 0; i < len(m.workers); i++ {
		idx := (m.nextWorker + i) % len(m.workers)
		if m.workers[idx].running {
			worker = m.workers[idx]
			m.nextWorker = (idx + 1) % len(m.workers)
			break
		}
	}
	if worker == nil {
		log.Printf("ERROR - Cannot submit job %s: no running workers", job.ID)
		return fmt.Errorf("no running workers available")
	}

	// Log job submission with sanitized params
	log.Printf("Submitting job %s (type=%s, worker=%d)", job.ID, job.Type, worker.id)
	log.Printf("Job %s params: steps=%v, cfg=%v, seed=%v",
		job.ID,
		job.Params["num_inference_steps"],
		job.Params["cfg_scale"],
		job.Params["seed"])

	msg := WorkerMessage{
		Type:  "job",
		JobID: job.ID,
	}
	data, err := json.Marshal(job)
	if err != nil {
		log.Printf("ERROR - Failed to marshal job %s: %v", job.ID, err)
		return fmt.Errorf("marshal job: %w", err)
	}
	msg.Data = data

	if err := json.NewEncoder(worker.stdin).Encode(msg); err != nil {
		log.Printf("ERROR - Failed to send job %s to worker %d: %v", job.ID, worker.id, err)
		return fmt.Errorf("send to worker: %w", err)
	}

	log.Printf("Job %s successfully sent to worker %d", job.ID, worker.id)
	return nil
}
