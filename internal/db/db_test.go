package db

import (
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db
}

func TestClearJobs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create some test jobs
	jobs := []*Job{
		{ID: "job-1", Type: "i2v", Status: "pending", Params: "{}"},
		{ID: "job-2", Type: "svi", Status: "running", Params: "{}"},
		{ID: "job-3", Type: "qwen", Status: "completed", Params: "{}"},
	}

	for _, job := range jobs {
		if err := db.CreateJob(job); err != nil {
			t.Fatalf("failed to create job: %v", err)
		}
	}

	// Verify jobs exist
	jobList, err := db.ListJobs(10)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobList) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobList))
	}

	// Clear all jobs
	if err := db.ClearJobs(); err != nil {
		t.Fatalf("failed to clear jobs: %v", err)
	}

	// Verify table is empty
	jobList, err = db.ListJobs(10)
	if err != nil {
		t.Fatalf("failed to list jobs after clear: %v", err)
	}
	if len(jobList) != 0 {
		t.Fatalf("expected 0 jobs after clear, got %d", len(jobList))
	}
}

func TestListJobs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create jobs with different timestamps
	now := time.Now()
	jobs := []*Job{
		{ID: "job-oldest", Type: "i2v", Status: "completed", Params: "{}"},
		{ID: "job-middle", Type: "svi", Status: "running", Params: "{}"},
		{ID: "job-newest", Type: "qwen", Status: "pending", Params: "{}"},
	}

	// Create jobs with slight delays to ensure different created_at timestamps
	for i, job := range jobs {
		// Manually set created_at with increasing timestamps
		_, err := db.conn.Exec(
			`INSERT INTO jobs (id, type, status, params, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			job.ID, job.Type, job.Status, job.Params,
			now.Add(time.Duration(i)*time.Second),
			now.Add(time.Duration(i)*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create job: %v", err)
		}
	}

	// Test listing all jobs - should be in DESC order (newest first)
	jobList, err := db.ListJobs(10)
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobList) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobList))
	}

	// Verify order: newest first
	if jobList[0].ID != "job-newest" {
		t.Errorf("expected first job to be job-newest, got %s", jobList[0].ID)
	}
	if jobList[1].ID != "job-middle" {
		t.Errorf("expected second job to be job-middle, got %s", jobList[1].ID)
	}
	if jobList[2].ID != "job-oldest" {
		t.Errorf("expected third job to be job-oldest, got %s", jobList[2].ID)
	}

	// Test limit
	limitedList, err := db.ListJobs(2)
	if err != nil {
		t.Fatalf("failed to list jobs with limit: %v", err)
	}
	if len(limitedList) != 2 {
		t.Fatalf("expected 2 jobs with limit, got %d", len(limitedList))
	}
	if limitedList[0].ID != "job-newest" {
		t.Errorf("expected first limited job to be job-newest, got %s", limitedList[0].ID)
	}

	// Test with limit 0 - should return empty
	emptyList, err := db.ListJobs(0)
	if err != nil {
		t.Fatalf("failed to list jobs with limit 0: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected 0 jobs with limit 0, got %d", len(emptyList))
	}
}

func TestListJobsWithNullFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a job with minimal fields (stage, output, error will be NULL)
	job := &Job{ID: "job-nulls", Type: "i2v", Status: "pending", Params: "{}"}
	if err := db.CreateJob(job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// List should handle NULL fields gracefully
	jobList, err := db.ListJobs(10)
	if err != nil {
		t.Fatalf("failed to list jobs with null fields: %v", err)
	}
	if len(jobList) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobList))
	}

	// Verify the job was retrieved with empty strings for null fields
	if jobList[0].Stage != "" {
		t.Errorf("expected empty stage for null field, got %s", jobList[0].Stage)
	}
	if jobList[0].Output != "" {
		t.Errorf("expected empty output for null field, got %s", jobList[0].Output)
	}
	if jobList[0].Error != "" {
		t.Errorf("expected empty error for null field, got %s", jobList[0].Error)
	}
}
