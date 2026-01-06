# Reliability Improvements Design

**Date:** 2026-01-06  
**Status:** Approved  
**Scope:** P0 and P1 architectural fixes from code review

## Overview

Address critical reliability issues identified in architectural review:
- Jobs lost on restart/refresh
- Failed jobs stuck in queue forever
- Worker crashes lose in-flight jobs
- No input validation
- Poor worker scheduling
- Frontend loses state on refresh

## Design Decisions

- **Job persistence:** Ephemeral (cleared on container restart, persists within session)
- **Retry policy:** One retry on failure, then mark failed
- **Input limits:** Conservative (10MB image, 500 char prompt)
- **Default updates:** Lightning LoRA optimized (I2V: 8 steps, Qwen: 4 steps, both 1.0 CFG)

---

## Section 1: Job Persistence (Ephemeral)

**Goal:** Jobs persist within a container session for crash recovery, cleared on restart.

**Changes:**

1. **`cmd/server/main.go`** - Add `database.ClearJobs()` on startup before queue consumer starts

2. **`internal/db/db.go`** - Add methods:
   - `ClearJobs()` - Truncate jobs table
   - `ListJobs(limit int)` - Return recent jobs for frontend sync

3. **`internal/api/workflows.go`** - After `queue.Enqueue()`, call `db.CreateJob()` to persist

4. **`cmd/server/main.go`** (worker callbacks) - Wire up:
   - Progress callback → `db.UpdateJobProgress()`
   - Complete callback → `db.CompleteJob()`
   - Error callback → `db.FailJob()`

5. **`internal/api/jobs.go`** - Implement TODO stubs to query database

**Data flow:**
```
Submit → Enqueue + DB.CreateJob → Worker → DB.UpdateProgress → Complete/Fail → DB.Complete/Fail
```

---

## Section 2: Queue Error Handling with Retry

**Goal:** Failed jobs get one retry before being marked failed. No ghost jobs.

**Changes:**

1. **`internal/queue/queue.go`** - Update `Consume()` error handling:

```go
if err := handler(message.ID, data); err != nil {
    retryCount, _ := data["retry_count"].(float64)
    jobID, _ := data["id"].(string)
    
    if int(retryCount) < 1 {
        // Requeue with incremented retry count
        data["retry_count"] = retryCount + 1
        q.Enqueue(stream.Stream, data)
        log.Printf("Job %s failed, requeuing (retry %d)", jobID, int(retryCount)+1)
    } else {
        // Max retries reached - caller marks failed
        log.Printf("Job %s failed after retry, marking failed", jobID)
    }
    // Always ACK to remove from pending
    q.client.XAck(ctx, stream.Stream, group, message.ID)
    continue
}
```

2. **`cmd/server/main.go`** - Update queue consumer to mark failed jobs in DB when max retries exceeded

---

## Section 3: Worker Crash Recovery

**Goal:** Auto-restart crashed workers and requeue their in-flight jobs.

**Changes:**

1. **`internal/worker/manager.go`** - Add to `Worker` struct:
   - `currentJobID string` - Track in-flight job

2. **`internal/worker/manager.go`** - Add to `Manager` struct:
   - `running bool` - Track if manager is active
   - `requeueCallback func(jobID string)` - Callback to requeue jobs

3. **`internal/worker/manager.go`** - Update `SubmitJob()`:
   - Set `worker.currentJobID = job.ID` before sending

4. **`internal/worker/manager.go`** - Update worker exit goroutine:

```go
go func() {
    err := cmd.Wait()
    worker.running = false
    
    if worker.currentJobID != "" {
        log.Printf("Worker %d crashed with job %s in-flight, requeuing", id, worker.currentJobID)
        m.requeueCallback(worker.currentJobID)
        worker.currentJobID = ""
    }
    
    // Auto-restart if manager still running
    if m.running {
        log.Printf("Restarting worker %d...", id)
        newWorker, err := m.spawnWorker(id)
        if err == nil {
            m.mu.Lock()
            m.workers[id] = newWorker
            m.mu.Unlock()
        }
    }
}()
```

5. **Worker callbacks** - Clear `currentJobID` on complete/error

---

## Section 4: Input Validation & Updated Defaults

**Goal:** Validate requests, enforce limits, update defaults for Lightning LoRAs.

**Changes:**

1. **`internal/api/workflows.go`** - Add validation:

```go
const (
    MaxImageSize    = 10 * 1024 * 1024  // 10MB base64
    MaxPromptLength = 500
)

func validateI2VRequest(req *I2VRequest) error {
    if len(req.InputImage) > MaxImageSize {
        return fmt.Errorf("input_image exceeds 10MB limit")
    }
    if len(req.Prompt) > MaxPromptLength {
        return fmt.Errorf("prompt exceeds 500 character limit")
    }
    if req.NumInferenceSteps < 1 || req.NumInferenceSteps > 100 {
        return fmt.Errorf("num_inference_steps must be 1-100")
    }
    if req.CFGScale < 0.1 || req.CFGScale > 20 {
        return fmt.Errorf("cfg_scale must be 0.1-20")
    }
    return nil
}
```

2. **`internal/api/workflows.go`** - Update I2V defaults:
   - `NumInferenceSteps`: 50 → **8**
   - `CFGScale`: 5.0 → **1.0**

3. **`internal/api/workflows.go`** - Update Qwen defaults:
   - `NumInferenceSteps`: 30 → **4**
   - `CFGScale`: 4.0 → **1.0**

4. **`web/src/components/I2VForm.tsx`** - Update form defaults:
   - `steps: 50` → `steps: 8`
   - `cfgScale: 5.0` → `cfgScale: 1.0`

5. **`web/src/components/QwenForm.tsx`** - Update form defaults:
   - `steps: 30` → `steps: 4`
   - `cfgScale: 4.0` → `cfgScale: 1.0`

6. **`internal/api/config_handlers.go`** - Update config export defaults to match

---

## Section 5: Worker Scheduling & Request Timeout

**Goal:** Distribute jobs across workers, prevent hung requests.

**Changes:**

1. **`internal/worker/manager.go`** - Add round-robin:

```go
type Manager struct {
    // ... existing fields
    nextWorker int
}

func (m *Manager) SubmitJob(job *JobRequest) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Round-robin to find next running worker
    for i := 0; i < len(m.workers); i++ {
        idx := (m.nextWorker + i) % len(m.workers)
        worker := m.workers[idx]
        if worker.running {
            m.nextWorker = (idx + 1) % len(m.workers)
            // ... submit to worker
        }
    }
    return fmt.Errorf("no running workers available")
}
```

2. **`internal/api/router.go`** - Add timeout middleware:

```go
r.Use(middleware.Timeout(60 * time.Second))
```

---

## Section 6: Frontend Job Sync on Load

**Goal:** Fetch existing jobs on page load so refreshes don't lose visibility.

**Changes:**

1. **`internal/api/jobs.go`** - Implement `handleListJobs()`:

```go
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
    jobs, err := s.db.ListJobs(50)
    if err != nil {
        http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(jobs)
}
```

2. **`internal/db/db.go`** - Add `ListJobs(limit int) ([]Job, error)`

3. **`web/src/api/jobs.ts`** - Add:

```typescript
export async function fetchJobs(): Promise<Job[]> {
    const res = await fetch('/api/jobs')
    return res.json()
}
```

4. **`web/src/stores/jobStore.ts`** - Add `setJobs(jobs: Job[])` action

5. **`web/src/App.tsx`** or **`WorkflowPage.tsx`** - Fetch on mount:

```typescript
useEffect(() => {
    fetchJobs().then(jobs => useJobStore.getState().setJobs(jobs))
}, [])
```

---

## Additional Fixes

- **`python/worker/qwen.py`** - Ensure `edit_image` is always passed as a list (per DiffSynth API requirement)

---

## Implementation Order

1. Section 1 (Job Persistence) - Foundation for everything else
2. Section 4 (Validation & Defaults) - Quick wins, no dependencies
3. Section 5 (Scheduling & Timeout) - Simple additions
4. Section 2 (Queue Retry) - Depends on Section 1 for marking failed
5. Section 3 (Worker Recovery) - Most complex, depends on queue working
6. Section 6 (Frontend Sync) - Depends on Section 1 for data
