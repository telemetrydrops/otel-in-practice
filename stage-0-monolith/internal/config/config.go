package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port         string `yaml:"port"`
	Host         string `yaml:"host"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	IdleTimeout  string `yaml:"idle_timeout"`
	Mode         string `yaml:"mode"` // debug, release, test
}

// DatabaseConfig contains database connection configuration
type DatabaseConfig struct {
	PostgreSQL PostgreSQLConfig `yaml:"postgresql"`
}

// PostgreSQLConfig contains PostgreSQL-specific configuration
type PostgreSQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
}

// Load loads configuration from a YAML file
func Load(configFile string) (*Config, error) {
	// Set defaults
	config := &Config{
		Server: ServerConfig{
			Port:         "8080",
			Host:         "0.0.0.0",
			ReadTimeout:  "15s",
			WriteTimeout: "15s",
			IdleTimeout:  "60s",
			Mode:         "debug",
		},
		Database: DatabaseConfig{
			PostgreSQL: PostgreSQLConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "postgres",
				Database: "postgres",
				SSLMode:  "disable",
			},
		},
	}

	// Read config file if it exists
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			if os.IsNotExist(err) {
				// Config file doesn't exist, use defaults
				return config, nil
			}
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// Expand environment variables
		expandedData := []byte(os.ExpandEnv(string(data)))

		// Parse YAML
		if err := yaml.Unmarshal(expandedData, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return config, nil
}

// GetDatabaseDSN returns a PostgreSQL connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.PostgreSQL.Host,
		c.Database.PostgreSQL.Port,
		c.Database.PostgreSQL.User,
		c.Database.PostgreSQL.Password,
		c.Database.PostgreSQL.Database,
		c.Database.PostgreSQL.SSLMode,
	)
}

// GetServerAddress returns the server address in host:port format
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}
