package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}

	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			base_model TEXT,
			author TEXT,
			description TEXT,
			tags TEXT,
			downloads INTEGER DEFAULT 0,
			rating REAL,
			nsfw INTEGER DEFAULT 0,
			files TEXT,
			thumbnail_url TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			synced_at DATETIME,
			local_path TEXT,
			local_size INTEGER,
			downloaded_at DATETIME,
			pinned INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_models_type ON models(type)`,
		`CREATE INDEX IF NOT EXISTS idx_models_base ON models(base_model)`,
		`CREATE INDEX IF NOT EXISTS idx_models_local ON models(local_path) WHERE local_path IS NOT NULL`,

		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			progress REAL DEFAULT 0,
			stage TEXT,
			params TEXT,
			output TEXT,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status)`,

		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS presets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			workflow TEXT NOT NULL,
			params TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, migration := range migrations {
		if _, err := db.conn.Exec(migration); err != nil {
			return err
		}
	}

	return nil
}

// Job methods

type Job struct {
	ID        string
	Type      string
	Status    string
	Progress  float64
	Stage     string
	Params    string
	Output    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (db *DB) CreateJob(job *Job) error {
	_, err := db.conn.Exec(
		`INSERT INTO jobs (id, type, status, params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		job.ID, job.Type, job.Status, job.Params, time.Now(), time.Now(),
	)
	return err
}

func (db *DB) GetJob(id string) (*Job, error) {
	job := &Job{}
	err := db.conn.QueryRow(
		`SELECT id, type, status, progress, stage, params, output, error, created_at, updated_at
		FROM jobs WHERE id = ?`,
		id,
	).Scan(&job.ID, &job.Type, &job.Status, &job.Progress, &job.Stage, &job.Params, &job.Output, &job.Error, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (db *DB) UpdateJobProgress(id string, progress float64, stage string) error {
	_, err := db.conn.Exec(
		`UPDATE jobs SET progress = ?, stage = ?, updated_at = ? WHERE id = ?`,
		progress, stage, time.Now(), id,
	)
	return err
}

func (db *DB) UpdateJobStatus(id string, status string) error {
	_, err := db.conn.Exec(
		`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (db *DB) CompleteJob(id string, output string) error {
	_, err := db.conn.Exec(
		`UPDATE jobs SET status = 'completed', output = ?, updated_at = ? WHERE id = ?`,
		output, time.Now(), id,
	)
	return err
}

func (db *DB) FailJob(id string, errorMsg string) error {
	_, err := db.conn.Exec(
		`UPDATE jobs SET status = 'failed', error = ?, updated_at = ? WHERE id = ?`,
		errorMsg, time.Now(), id,
	)
	return err
}

// Config methods

func (db *DB) GetConfig(key string) (string, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (db *DB) SetConfig(key, value string) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO config (key, value, updated_at) VALUES (?, ?, ?)`,
		key, value, time.Now(),
	)
	return err
}
