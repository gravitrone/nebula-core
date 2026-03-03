package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var marshalConfigYAML = yaml.Marshal

// Config holds CLI configuration stored at ~/.nebula/config.
type Config struct {
	APIKey            string `yaml:"api_key"`
	UserEntityID      string `yaml:"user_entity_id"`
	Username          string `yaml:"username"`
	Theme             string `yaml:"theme"`
	VimKeys           bool   `yaml:"vim_keys"`
	QuickstartPending bool   `yaml:"quickstart_pending,omitempty"`
	PendingLimit      int    `yaml:"pending_limit,omitempty"`
}

// Path returns the config file path.
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nebula", "config")
}

// Load reads and parses the config file. Returns error if missing or insecure.
func Load() (*Config, error) {
	path := Path()

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("config not found: %w", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		return nil, fmt.Errorf("config permissions too open: %04o (want 0600)", perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("config missing api_key")
	}
	if cfg.PendingLimit <= 0 {
		cfg.PendingLimit = 500
	}

	return &cfg, nil
}

// Save writes the config to disk with secure permissions.
func (c *Config) Save() error {
	path := Path()
	dir := filepath.Dir(path)
	if c.PendingLimit <= 0 {
		c.PendingLimit = 500
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := marshalConfigYAML(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}
