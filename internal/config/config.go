package config

import (
	"os"
)

type Config struct {
	Port       string
	DataDir    string
	ModelsDir  string
	OutputsDir string
	StaticDir  string

	ValkeyAddr string
	ValkeyPort string

	Aria2Port           string
	Aria2MaxConnections int

	WorkerCount int
	PythonPath  string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:       getEnv("DIFFBOX_PORT", "8080"),
		DataDir:    getEnv("DIFFBOX_DATA_DIR", "./data"),
		ModelsDir:  getEnv("DIFFBOX_MODELS_DIR", "./models"),
		OutputsDir: getEnv("DIFFBOX_OUTPUTS_DIR", "./outputs"),
		StaticDir:  getEnv("DIFFBOX_STATIC_DIR", "./web/dist"),

		ValkeyAddr: getEnv("DIFFBOX_VALKEY_ADDR", "localhost:6379"),
		ValkeyPort: getEnv("DIFFBOX_VALKEY_PORT", "6379"),

		Aria2Port:           getEnv("DIFFBOX_ARIA2_PORT", "6800"),
		Aria2MaxConnections: 16,

		WorkerCount: 1,
		PythonPath:  getEnv("DIFFBOX_PYTHON_PATH", "./python"),
	}

	// Ensure directories exist
	dirs := []string{cfg.DataDir, cfg.ModelsDir, cfg.OutputsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
