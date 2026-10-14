package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the catalog service configuration.
type Config struct {
	GRPCPort string `yaml:"grpc_port"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
}

// Load reads YAML from file, then overlays environment variables for fields
// that have known overrides (DATABASE_URL, GRPC_PORT).
func Load(file string) (*Config, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("GRPC_PORT"); v != "" {
		cfg.GRPCPort = v
	}
	return &cfg, nil
}
