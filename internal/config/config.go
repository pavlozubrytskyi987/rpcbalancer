// Package config loads and validates the YAML config.
package config

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the whole config file.
type Config struct {
	Listen   string    `yaml:"listen"`
	Backends []Backend `yaml:"backends"`
}

// Backend is one backend entry in the YAML.
type Backend struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Load reads path, parses the YAML and validates it.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if len(c.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}
	seen := map[string]struct{}{}
	for i, b := range c.Backends {
		if b.Name == "" {
			return fmt.Errorf("backend %d: name is empty", i)
		}
		if _, dup := seen[b.Name]; dup {
			return fmt.Errorf("backend %d: duplicate name %q", i, b.Name)
		}
		seen[b.Name] = struct{}{}
		u, err := url.Parse(b.URL)
		if err != nil {
			return fmt.Errorf("backend %q: parse url: %w", b.Name, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("backend %q: url scheme must be http or https, got %q", b.Name, u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("backend %q: url host is empty", b.Name)
		}
	}
	return nil
}
