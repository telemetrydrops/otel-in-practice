package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the order service configuration.
type Config struct {
	HTTPPort string `yaml:"http_port"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Catalog struct {
		Address string `yaml:"address"`
	} `yaml:"catalog"`
}

// Load reads YAML, then overlays env vars.
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
	if v := os.Getenv("HTTP_PORT"); v != "" {
		cfg.HTTPPort = v
	}
	if v := os.Getenv("CATALOG_GRPC_ADDR"); v != "" {
		cfg.Catalog.Address = v
	}
	return &cfg, nil
}
