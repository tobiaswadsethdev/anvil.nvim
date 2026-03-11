package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Filter represents a named JQL filter.
type Filter struct {
	Name string `yaml:"name"`
	JQL  string `yaml:"jql"`
}

// JiraConfig holds Jira Cloud connection settings.
type JiraConfig struct {
	URL   string `yaml:"url"`
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

// Config is the root configuration structure.
type Config struct {
	Jira    JiraConfig `yaml:"jira"`
	Filters []Filter   `yaml:"filters"`
}

// DefaultConfigPath returns ~/.config/anvil/config.yaml.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "anvil", "config.yaml")
}

// Load reads the config file and applies environment variable overrides.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Environment variable overrides.
	if v := os.Getenv("JIRA_URL"); v != "" {
		cfg.Jira.URL = v
	}
	if v := os.Getenv("JIRA_USER"); v != "" {
		cfg.Jira.User = v
	}
	if v := os.Getenv("JIRA_TOKEN"); v != "" {
		cfg.Jira.Token = v
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Jira.URL == "" {
		return fmt.Errorf("jira.url is required (set in config or JIRA_URL env var)")
	}
	if cfg.Jira.User == "" {
		return fmt.Errorf("jira.user is required (set in config or JIRA_USER env var)")
	}
	if cfg.Jira.Token == "" {
		return fmt.Errorf("jira.token is required (set in config or JIRA_TOKEN env var)")
	}
	if len(cfg.Filters) == 0 {
		cfg.Filters = []Filter{
			{
				Name: "All Issues",
				JQL:  "assignee = currentUser() ORDER BY updated DESC",
			},
		}
	}
	return nil
}
