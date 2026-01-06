# Reliability Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement P0/P1 reliability fixes: job persistence, queue retry, worker recovery, input validation, and frontend sync.

**Architecture:** Jobs are persisted to SQLite (ephemeral per container session), queue failures trigger one retry then mark failed, crashed workers auto-restart and requeue in-flight jobs, requests are validated before processing, and frontend fetches existing jobs on load.

**Tech Stack:** Go 1.23, SQLite, Redis Streams, React 19, TypeScript, Zustand

---

## Task 1: Database - Add ClearJobs and ListJobs Methods

**Files:**
- Modify: `internal/db/db.go`
- Create: `internal/db/db_test.go`

**Step 1: Write failing tests for ClearJobs and ListJobs**

Create `internal/db/db_test.go`:

```go
package db

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestClearJobs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create some jobs
	job1 := &Job{ID: "job-1", Type: "i2v", Status: "pending", Params: "{}"}
	job2 := &Job{ID: "job-2", Type: "qwen", Status: "running", Params: "{}"}

	if err := db.CreateJob(job1); err != nil {
		t.Fatalf("Failed to create job1: %v", err)
	}
	if err := db.CreateJob(job2); err != nil {
		t.Fatalf("Failed to create job2: %v", err)
	}

	// Clear jobs
	if err := db.ClearJobs(); err != nil {
		t.Fatalf("ClearJobs failed: %v", err)
	}

	// Verify jobs are cleared
	jobs, err := db.ListJobs(10)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs after clear, got %d", len(jobs))
	}
}

func TestListJobs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create jobs with different timestamps
	for i := 1; i <= 5; i++ {
		job := &Job{
			ID:     fmt.Sprintf("job-%d", i),
			Type:   "i2v",
			Status: "pending",
			Params: "{}",
		}
		if err := db.CreateJob(job); err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List with limit
	jobs, err := db.ListJobs(3)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(jobs))
	}

	// Should be in reverse chronological order (newest first)
	if jobs[0].ID != "job-5" {
		t.Errorf("Expected newest job first, got %s", jobs[0].ID)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/db/...`
Expected: FAIL - missing methods and import

**Step 3: Add fmt import and implement ClearJobs and ListJobs**

Add to `internal/db/db.go` after the existing methods:

```go
// ClearJobs removes all jobs from the database (used on startup for ephemeral mode)
func (db *DB) ClearJobs() error {
	_, err := db.conn.Exec(`DELETE FROM jobs`)
	return err
}

// ListJobs returns the most recent jobs up to the specified limit
func (db *DB) ListJobs(limit int) ([]Job, error) {
	rows, err := db.conn.Query(
		`SELECT id, type, status, progress, COALESCE(stage, ''), COALESCE(params, '{}'), 
		        COALESCE(output, ''), COALESCE(error, ''), created_at, updated_at
		 FROM jobs ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		err := rows.Scan(&job.ID, &job.Type, &job.Status, &job.Progress, &job.Stage,
			&job.Params, &job.Output, &job.Error, &job.CreatedAt, &job.UpdatedAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
```

Also add `"fmt"` to the test file imports.

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/db/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go
git commit -m "feat(db): add ClearJobs and ListJobs methods for ephemeral persistence"
```

---

## Task 2: Wire Up Job Persistence in Workflows

**Files:**
- Modify: `internal/api/workflows.go`
- Modify: `internal/api/server.go` (if exists) or `internal/api/router.go`

**Step 1: Update handleI2VSubmit to persist job to database**

In `internal/api/workflows.go`, after the `queue.Enqueue()` call in `handleI2VSubmit`, add:

```go
	// Persist job to database
	dbJob := &db.Job{
		ID:     jobID,
		Type:   "i2v",
		Status: "pending",
		Params: string(mustMarshal(req)),
	}
	if err := s.db.CreateJob(dbJob); err != nil {
		log.Printf("Warning: Failed to persist job %s to database: %v", jobID, err)
		// Don't fail the request - job is already queued
	}
```

Add helper function at the top of the file:

```go
func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
```

**Step 2: Update handleSVISubmit similarly**

After `queue.Enqueue()`:

```go
	// Persist job to database
	dbJob := &db.Job{
		ID:     jobID,
		Type:   "svi",
		Status: "pending",
		Params: string(mustMarshal(req)),
	}
	if err := s.db.CreateJob(dbJob); err != nil {
		log.Printf("Warning: Failed to persist job %s to database: %v", jobID, err)
	}
```

**Step 3: Update handleQwenSubmit similarly**

After `queue.Enqueue()`:

```go
	// Persist job to database
	dbJob := &db.Job{
		ID:     jobID,
		Type:   "qwen",
		Status: "pending",
		Params: string(mustMarshal(req)),
	}
	if err := s.db.CreateJob(dbJob); err != nil {
		log.Printf("Warning: Failed to persist job %s to database: %v", jobID, err)
	}
```

**Step 4: Add db import**

Add `"github.com/druarnfield/diffbox/internal/db"` to imports.

**Step 5: Run linter and tests**

Run: `make lint && make test`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/workflows.go
git commit -m "feat(api): persist jobs to database on submission"
```

---

## Task 3: Wire Up Progress/Complete/Error Callbacks to Database

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Update worker callbacks to persist state**

In `cmd/server/main.go`, find the `workerManager.SetCallbacks` section and update:

```go
	// Wire up worker callbacks to WebSocket hub AND database
	workerManager.SetCallbacks(
		// Progress callback
		func(progress worker.ProgressUpdate) {
			// Update database
			if err := database.UpdateJobProgress(progress.JobID, progress.Progress, progress.Stage); err != nil {
				log.Printf("Warning: Failed to update job progress in DB: %v", err)
			}
			// Broadcast to WebSocket
			wsHub.BroadcastJobProgress(api.JobProgress{
				JobID:    progress.JobID,
				Progress: progress.Progress,
				Stage:    progress.Stage,
				Preview:  progress.Preview,
			})
		},
		// Complete callback
		func(result worker.JobResult) {
			// Update database
			if err := database.CompleteJob(result.JobID, result.Output); err != nil {
				log.Printf("Warning: Failed to mark job complete in DB: %v", err)
			}
			// Broadcast to WebSocket
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
			// Update database
			if err := database.FailJob(result.JobID, result.Error); err != nil {
				log.Printf("Warning: Failed to mark job failed in DB: %v", err)
			}
			// Broadcast to WebSocket
			wsHub.BroadcastJobError(api.JobError{
				JobID: result.JobID,
				Error: result.Error,
			})
		},
	)
```

**Step 2: Run linter and tests**

Run: `make lint && make test`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): persist job progress/complete/error to database"
```

---

## Task 4: Clear Jobs on Startup

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add ClearJobs call after database initialization**

In `cmd/server/main.go`, after `database, err := db.New(...)` and before starting the queue consumer:

```go
	// Clear jobs from previous session (ephemeral mode)
	if err := database.ClearJobs(); err != nil {
		log.Printf("Warning: Failed to clear jobs: %v", err)
	} else {
		log.Println("Cleared jobs from previous session")
	}
```

**Step 2: Run and verify**

Run: `make build && make lint`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): clear jobs on startup for ephemeral mode"
```

---

## Task 5: Implement Jobs API Handlers

**Files:**
- Modify: `internal/api/jobs.go`

**Step 1: Implement handleListJobs**

Replace the TODO stub:

```go
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.db.ListJobs(50)
	if err != nil {
		log.Printf("Failed to list jobs: %v", err)
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	// Convert db.Job to api.Job
	apiJobs := make([]Job, len(jobs))
	for i, job := range jobs {
		var params map[string]interface{}
		json.Unmarshal([]byte(job.Params), &params)

		apiJobs[i] = Job{
			ID:        job.ID,
			Type:      job.Type,
			Status:    job.Status,
			Progress:  job.Progress,
			Stage:     job.Stage,
			Params:    params,
			Error:     job.Error,
			CreatedAt: job.CreatedAt.Format(time.RFC3339),
			UpdatedAt: job.UpdatedAt.Format(time.RFC3339),
		}

		if job.Output != "" {
			var output JobOutput
			if err := json.Unmarshal([]byte(job.Output), &output); err == nil {
				apiJobs[i].Output = &output
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiJobs)
}
```

**Step 2: Implement handleGetJob**

Replace the TODO stub:

```go
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	job, err := s.db.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	var params map[string]interface{}
	json.Unmarshal([]byte(job.Params), &params)

	apiJob := Job{
		ID:        job.ID,
		Type:      job.Type,
		Status:    job.Status,
		Progress:  job.Progress,
		Stage:     job.Stage,
		Params:    params,
		Error:     job.Error,
		CreatedAt: job.CreatedAt.Format(time.RFC3339),
		UpdatedAt: job.UpdatedAt.Format(time.RFC3339),
	}

	if job.Output != "" {
		var output JobOutput
		if err := json.Unmarshal([]byte(job.Output), &output); err == nil {
			apiJob.Output = &output
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiJob)
}
```

**Step 3: Add time import**

Add `"time"` to the imports.

**Step 4: Run linter**

Run: `make lint`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/jobs.go
git commit -m "feat(api): implement job listing and fetching from database"
```

---

## Task 6: Update Lightning LoRA Defaults (Backend)

**Files:**
- Modify: `internal/api/workflows.go`
- Modify: `internal/api/config_handlers.go`

**Step 1: Update I2V defaults in workflows.go**

Find the I2V defaults section and change:

```go
	if req.NumInferenceSteps == 0 {
		req.NumInferenceSteps = 8  // Lightning LoRA optimized
	}
	if req.CFGScale == 0 {
		req.CFGScale = 1.0  // Lightning LoRA optimized
	}
```

**Step 2: Update Qwen defaults in workflows.go**

Find the Qwen defaults section and change:

```go
	if req.NumInferenceSteps == 0 {
		req.NumInferenceSteps = 4  // 4-step Lightning LoRA
	}
	if req.CFGScale == 0 {
		req.CFGScale = 1.0  // Lightning LoRA optimized
	}
```

**Step 3: Update config_handlers.go defaults**

Find the config export defaults and update I2V:

```go
"num_inference_steps": 8,
"cfg_scale":           1.0,
```

And Qwen:

```go
"num_inference_steps": 4,
"cfg_scale":           1.0,
```

**Step 4: Run linter**

Run: `make lint`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/workflows.go internal/api/config_handlers.go
git commit -m "feat(api): update defaults for Lightning LoRA (I2V: 8 steps, Qwen: 4 steps, CFG 1.0)"
```

---

## Task 7: Update Lightning LoRA Defaults (Frontend)

**Files:**
- Modify: `web/src/components/I2VForm.tsx`
- Modify: `web/src/components/QwenForm.tsx`

**Step 1: Update I2VForm defaults**

In `I2VForm.tsx`, find the initial state and change:

```typescript
    steps: 8,
    cfgScale: 1.0,
```

Also update the fallback in onChange handlers:

```typescript
onChange={(e) => setForm((s) => ({ ...s, steps: parseInt(e.target.value, 10) || 8 }))}
```

```typescript
onChange={(e) => setForm((s) => ({ ...s, cfgScale: parseFloat(e.target.value) || 1.0 }))}
```

**Step 2: Update QwenForm defaults**

In `QwenForm.tsx`, find the initial state and change:

```typescript
    steps: 4,
    cfgScale: 1.0,
```

Also update the fallback in onChange handlers:

```typescript
onChange={(e) => setForm((s) => ({ ...s, steps: parseInt(e.target.value, 10) || 4 }))}
```

```typescript
onChange={(e) => setForm((s) => ({ ...s, cfgScale: parseFloat(e.target.value) || 1.0 }))}
```

**Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 4: Commit**

```bash
git add web/src/components/I2VForm.tsx web/src/components/QwenForm.tsx
git commit -m "feat(web): update form defaults for Lightning LoRA"
```

---

## Task 8: Add Input Validation

**Files:**
- Modify: `internal/api/workflows.go`

**Step 1: Add validation constants and function**

At the top of `workflows.go`, after imports, add:

```go
const (
	MaxImageSize    = 10 * 1024 * 1024 // 10MB base64
	MaxPromptLength = 500
	MaxSteps        = 100
	MinSteps        = 1
	MaxCFGScale     = 20.0
	MinCFGScale     = 0.1
)

func validateI2VRequest(req *I2VRequest) error {
	if len(req.InputImage) > MaxImageSize {
		return fmt.Errorf("input_image exceeds 10MB limit (%d bytes)", len(req.InputImage))
	}
	if len(req.Prompt) > MaxPromptLength {
		return fmt.Errorf("prompt exceeds %d character limit", MaxPromptLength)
	}
	if req.NumInferenceSteps != 0 && (req.NumInferenceSteps < MinSteps || req.NumInferenceSteps > MaxSteps) {
		return fmt.Errorf("num_inference_steps must be %d-%d", MinSteps, MaxSteps)
	}
	if req.CFGScale != 0 && (req.CFGScale < MinCFGScale || req.CFGScale > MaxCFGScale) {
		return fmt.Errorf("cfg_scale must be %.1f-%.1f", MinCFGScale, MaxCFGScale)
	}
	return nil
}

func validateQwenRequest(req *QwenRequest) error {
	totalImageSize := 0
	for _, img := range req.EditImages {
		totalImageSize += len(img)
	}
	if totalImageSize > MaxImageSize {
		return fmt.Errorf("total edit_images size exceeds 10MB limit (%d bytes)", totalImageSize)
	}
	if len(req.Prompt) > MaxPromptLength {
		return fmt.Errorf("prompt exceeds %d character limit", MaxPromptLength)
	}
	if req.NumInferenceSteps != 0 && (req.NumInferenceSteps < MinSteps || req.NumInferenceSteps > MaxSteps) {
		return fmt.Errorf("num_inference_steps must be %d-%d", MinSteps, MaxSteps)
	}
	if req.CFGScale != 0 && (req.CFGScale < MinCFGScale || req.CFGScale > MaxCFGScale) {
		return fmt.Errorf("cfg_scale must be %.1f-%.1f", MinCFGScale, MaxCFGScale)
	}
	return nil
}
```

**Step 2: Add validation call in handleI2VSubmit**

After decoding the request, before setting defaults:

```go
	// Validate request
	if err := validateI2VRequest(&req); err != nil {
		log.Printf("I2V: Validation failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
```

**Step 3: Add validation call in handleQwenSubmit**

After decoding the request, before setting defaults:

```go
	// Validate request
	if err := validateQwenRequest(&req); err != nil {
		log.Printf("Qwen: Validation failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
```

**Step 4: Add fmt import if not present**

**Step 5: Run linter and tests**

Run: `make lint && make test`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/workflows.go
git commit -m "feat(api): add input validation for workflow requests"
```

---

## Task 9: Add Request Timeout Middleware

**Files:**
- Modify: `internal/api/router.go`

**Step 1: Add timeout middleware**

In `NewRouter()`, after the existing middleware:

```go
	r.Use(middleware.Timeout(60 * time.Second))
```

**Step 2: Add time import**

Add `"time"` to imports.

**Step 3: Run linter**

Run: `make lint`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(api): add 60-second request timeout middleware"
```

---

## Task 10: Add Round-Robin Worker Scheduling

**Files:**
- Modify: `internal/worker/manager.go`
- Modify: `internal/worker/manager_test.go`

**Step 1: Add nextWorker field to Manager struct**

In the `Manager` struct:

```go
type Manager struct {
	cfg              *config.Config
	workers          []*Worker
	mu               sync.Mutex
	onProgress       ProgressCallback
	onComplete       CompleteCallback
	onError          ErrorCallback
	nextWorker       int  // Round-robin index
}
```

**Step 2: Update SubmitJob for round-robin**

Replace the current `SubmitJob` implementation:

```go
func (m *Manager) SubmitJob(job *JobRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.workers) == 0 {
		log.Printf("ERROR - Cannot submit job %s: no workers available", job.ID)
		return fmt.Errorf("no workers available")
	}

	// Round-robin to find next running worker
	var worker *Worker
	for i := 0; i < len(m.workers); i++ {
		idx := (m.nextWorker + i) % len(m.workers)
		w := m.workers[idx]
		if w.running {
			worker = w
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
```

**Step 3: Run linter and tests**

Run: `make lint && make test`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/worker/manager.go
git commit -m "feat(worker): add round-robin scheduling for multiple workers"
```

---

## Task 11: Add Queue Retry Logic

**Files:**
- Modify: `internal/queue/queue.go`

**Step 1: Update Consume to handle retries**

Replace the error handling section in `Consume()`:

```go
func (q *RedisQueue) Consume(stream string, group string, consumer string, handler func(id string, data map[string]interface{}) error) error {
	// Create consumer group if not exists
	q.client.XGroupCreateMkStream(q.ctx, stream, group, "0")

	for {
		streams, err := q.client.XReadGroup(q.ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    0,
		}).Result()

		if err != nil {
			return err
		}

		for _, s := range streams {
			for _, message := range s.Messages {
				dataStr, ok := message.Values["data"].(string)
				if !ok {
					continue
				}

				var data map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					log.Printf("ERROR - Failed to unmarshal job data from queue: %v", err)
					q.client.XAck(q.ctx, s.Stream, group, message.ID)
					continue
				}

				jobID, _ := data["id"].(string)

				if err := handler(message.ID, data); err != nil {
					retryCount, _ := data["retry_count"].(float64)

					if int(retryCount) < 1 {
						// Requeue with incremented retry count
						data["retry_count"] = retryCount + 1
						log.Printf("Job %s failed, requeuing (retry %d): %v", jobID, int(retryCount)+1, err)
						q.Enqueue(s.Stream, data)
					} else {
						// Max retries reached
						log.Printf("ERROR - Job %s failed after retry, giving up: %v", jobID, err)
					}

					// Always ACK to remove from pending
					q.client.XAck(q.ctx, s.Stream, group, message.ID)
					continue
				}

				// Acknowledge successful message
				q.client.XAck(q.ctx, s.Stream, group, message.ID)
				log.Printf("Job %s acknowledged and removed from queue", jobID)
			}
		}
	}
}
```

**Step 2: Run linter**

Run: `make lint`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/queue/queue.go
git commit -m "feat(queue): add retry logic with max 1 retry before failing"
```

---

## Task 12: Add Frontend Jobs API Client

**Files:**
- Create: `web/src/api/jobs.ts`

**Step 1: Create jobs API client**

Create `web/src/api/jobs.ts`:

```typescript
import type { Job } from '@/stores/jobStore'

export interface ApiJob {
  id: string
  type: 'i2v' | 'qwen'
  status: 'pending' | 'running' | 'completed' | 'failed'
  progress: number
  stage: string
  params: Record<string, unknown>
  output?: {
    type: string
    path: string
    frames?: number
  }
  error?: string
  created_at: string
  updated_at: string
}

export async function fetchJobs(): Promise<Job[]> {
  const res = await fetch('/api/jobs')
  if (!res.ok) {
    throw new Error(`Failed to fetch jobs: ${res.status}`)
  }
  const apiJobs: ApiJob[] = await res.json()

  return apiJobs.map((job) => ({
    id: job.id,
    type: job.type,
    status: job.status,
    progress: job.progress,
    stage: job.stage,
    preview: undefined,
    output: job.output
      ? {
          type: job.output.type as 'video' | 'image',
          path: job.output.path,
          frames: job.output.frames,
        }
      : undefined,
    error: job.error,
    params: job.params,
    createdAt: new Date(job.created_at),
  }))
}
```

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/api/jobs.ts
git commit -m "feat(web): add jobs API client for fetching existing jobs"
```

---

## Task 13: Add setJobs Action to Store

**Files:**
- Modify: `web/src/stores/jobStore.ts`

**Step 1: Add setJobs action**

In the store interface and implementation:

```typescript
interface JobStore {
  jobs: Job[]
  activeJobId: string | null

  addJob: (job: Omit<Job, 'createdAt'>) => void
  setJobs: (jobs: Job[]) => void  // Add this
  updateJobProgress: (jobId: string, progress: number, stage: string, preview?: string) => void
  completeJob: (jobId: string, output: JobOutput) => void
  failJob: (jobId: string, error: string) => void
  removeJob: (jobId: string) => void
  setActiveJob: (jobId: string | null) => void
  getJob: (jobId: string) => Job | undefined
}
```

And add the implementation:

```typescript
  setJobs: (jobs) => {
    set({ jobs, activeJobId: jobs.length > 0 ? jobs[0].id : null })
  },
```

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/stores/jobStore.ts
git commit -m "feat(web): add setJobs action to job store"
```

---

## Task 14: Fetch Jobs on Page Load

**Files:**
- Modify: `web/src/pages/WorkflowPage.tsx`

**Step 1: Add useEffect to fetch jobs on mount**

Add imports:

```typescript
import { useEffect } from 'react'
import { fetchJobs } from '@/api/jobs'
import { useJobStore } from '@/stores/jobStore'
```

Add effect at the start of the component:

```typescript
export default function WorkflowPage() {
  // Fetch existing jobs on mount
  useEffect(() => {
    fetchJobs()
      .then((jobs) => {
        useJobStore.getState().setJobs(jobs)
      })
      .catch((err) => {
        console.error('Failed to fetch jobs:', err)
      })
  }, [])

  // ... rest of component
```

**Step 2: Run frontend lint and build**

Run: `cd web && npm run lint && npm run build`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/pages/WorkflowPage.tsx
git commit -m "feat(web): fetch existing jobs on page load"
```

---

## Task 15: Final Verification

**Step 1: Run full test suite**

Run: `make lint && make test`
Expected: All tests pass

**Step 2: Build frontend**

Run: `cd web && npm run build`
Expected: Build succeeds

**Step 3: Build Go binary**

Run: `make build`
Expected: Build succeeds

**Step 4: Final commit for any remaining changes**

```bash
git status
# If any uncommitted changes:
git add -A
git commit -m "chore: final cleanup for reliability improvements"
```

**Step 5: Push all changes**

```bash
git push
```

---

## Summary of Changes

| Component | Changes |
|-----------|---------|
| Database | ClearJobs(), ListJobs() methods |
| Workflows | Job persistence, input validation, updated defaults |
| Server | Clear jobs on startup, persist progress/complete/error |
| Jobs API | Implemented list and get handlers |
| Queue | Retry logic (1 retry max) |
| Worker | Round-robin scheduling |
| Router | 60s request timeout |
| Frontend | Jobs API client, setJobs action, fetch on load, updated defaults |
