package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/druarnfield/diffbox/internal/config"
)

type Manager struct {
	cfg       *config.Config
	workers   []*Worker
	mu        sync.Mutex
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
	Type    string          `json:"type"`
	JobID   string          `json:"job_id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
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

	log.Printf("Worker %d started (PID: %d)", id, cmd.Process.Pid)

	return worker, nil
}

func (m *Manager) handleWorkerOutput(w *Worker) {
	scanner := bufio.NewScanner(w.stdout)
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
			json.Unmarshal(msg.Data, &progress)
			// TODO: Broadcast via WebSocket hub
			log.Printf("Worker %d: job %s progress %.1f%% - %s", w.id, progress.JobID, progress.Progress*100, progress.Stage)

		case "complete":
			var result JobResult
			json.Unmarshal(msg.Data, &result)
			// TODO: Update job in database, broadcast via WebSocket
			log.Printf("Worker %d: job %s completed: %s", w.id, result.JobID, result.Output)

		case "error":
			var result JobResult
			json.Unmarshal(msg.Data, &result)
			// TODO: Update job in database, broadcast via WebSocket
			log.Printf("Worker %d: job %s failed: %s", w.id, result.JobID, result.Error)

		case "ready":
			log.Printf("Worker %d: ready", w.id)
		}
	}
}

func (m *Manager) handleWorkerLogs(w *Worker) {
	scanner := bufio.NewScanner(w.stderr)
	for scanner.Scan() {
		log.Printf("Worker %d [stderr]: %s", w.id, scanner.Text())
	}
}

func (m *Manager) SubmitJob(job *JobRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find an available worker (simple round-robin for now)
	if len(m.workers) == 0 {
		return fmt.Errorf("no workers available")
	}

	worker := m.workers[0] // TODO: Implement proper scheduling

	msg := WorkerMessage{
		Type:  "job",
		JobID: job.ID,
	}
	data, _ := json.Marshal(job)
	msg.Data = data

	return json.NewEncoder(worker.stdin).Encode(msg)
}
