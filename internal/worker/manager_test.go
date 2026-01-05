package worker

import (
	"encoding/json"
	"testing"

	"github.com/druarnfield/diffbox/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		WorkerCount: 2,
	}

	manager := NewManager(cfg)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.cfg != cfg {
		t.Error("manager config not set correctly")
	}

	if len(manager.workers) != 0 {
		t.Errorf("expected 0 workers initially, got %d", len(manager.workers))
	}
}

func TestSetCallbacks(t *testing.T) {
	cfg := &config.Config{}
	manager := NewManager(cfg)

	progressCalled := false
	completeCalled := false
	errorCalled := false

	manager.SetCallbacks(
		func(p ProgressUpdate) { progressCalled = true },
		func(r JobResult) { completeCalled = true },
		func(r JobResult) { errorCalled = true },
	)

	// Test that callbacks are set
	if manager.onProgress == nil {
		t.Error("onProgress callback not set")
	}
	if manager.onComplete == nil {
		t.Error("onComplete callback not set")
	}
	if manager.onError == nil {
		t.Error("onError callback not set")
	}

	// Test that callbacks work
	manager.onProgress(ProgressUpdate{})
	manager.onComplete(JobResult{})
	manager.onError(JobResult{})

	if !progressCalled {
		t.Error("progress callback not called")
	}
	if !completeCalled {
		t.Error("complete callback not called")
	}
	if !errorCalled {
		t.Error("error callback not called")
	}
}

func TestWorkerMessageSerialization(t *testing.T) {
	msg := WorkerMessage{
		Type:  "job",
		JobID: "test-123",
		Data:  json.RawMessage(`{"key": "value"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal WorkerMessage: %v", err)
	}

	var decoded WorkerMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal WorkerMessage: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: got %s, expected %s", decoded.Type, msg.Type)
	}
	if decoded.JobID != msg.JobID {
		t.Errorf("JobID mismatch: got %s, expected %s", decoded.JobID, msg.JobID)
	}
}

func TestJobRequestSerialization(t *testing.T) {
	job := JobRequest{
		ID:   "job-456",
		Type: "i2v",
		Params: map[string]interface{}{
			"prompt": "test prompt",
			"seed":   42,
		},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal JobRequest: %v", err)
	}

	var decoded JobRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JobRequest: %v", err)
	}

	if decoded.ID != job.ID {
		t.Errorf("ID mismatch: got %s, expected %s", decoded.ID, job.ID)
	}
	if decoded.Type != job.Type {
		t.Errorf("Type mismatch: got %s, expected %s", decoded.Type, job.Type)
	}
	if decoded.Params["prompt"] != "test prompt" {
		t.Errorf("Params prompt mismatch")
	}
}

func TestProgressUpdateSerialization(t *testing.T) {
	progress := ProgressUpdate{
		JobID:    "job-789",
		Progress: 0.5,
		Stage:    "Denoising step 25/50",
		Preview:  "base64data...",
	}

	data, err := json.Marshal(progress)
	if err != nil {
		t.Fatalf("failed to marshal ProgressUpdate: %v", err)
	}

	var decoded ProgressUpdate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ProgressUpdate: %v", err)
	}

	if decoded.Progress != progress.Progress {
		t.Errorf("Progress mismatch: got %f, expected %f", decoded.Progress, progress.Progress)
	}
	if decoded.Stage != progress.Stage {
		t.Errorf("Stage mismatch: got %s, expected %s", decoded.Stage, progress.Stage)
	}
}

func TestJobResultSerialization(t *testing.T) {
	result := JobResult{
		JobID:  "job-999",
		Status: "completed",
		Output: "/outputs/job-999.mp4",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal JobResult: %v", err)
	}

	var decoded JobResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JobResult: %v", err)
	}

	if decoded.Status != result.Status {
		t.Errorf("Status mismatch: got %s, expected %s", decoded.Status, result.Status)
	}
	if decoded.Output != result.Output {
		t.Errorf("Output mismatch: got %s, expected %s", decoded.Output, result.Output)
	}
}
