package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the server configuration, sourced from environment variables.
type Config struct {
	Port     string
	DataDir  string
	APIToken string // empty => auth disabled (LAN-only mode)
}

// Load reads configuration from the environment with sane defaults.
func Load() *Config {
	return &Config{
		Port:     getenv("CS_PORT", "8090"),
		DataDir:  getenv("CS_DATA_DIR", "./data"),
		APIToken: os.Getenv("CS_API_TOKEN"),
	}
}

// Validate ensures required fields are usable.
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("port must not be empty")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data dir must not be empty")
	}
	return nil
}

// SavesDir returns the directory where uploaded save files are stored.
func (c *Config) SavesDir() string {
	return filepath.Join(c.DataDir, "saves")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
