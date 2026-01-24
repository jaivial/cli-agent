package app

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MinimaxAPIKey     string `yaml:"minimax_api_key"`
	BaseURL           string `yaml:"base_url"`
	Model             string `yaml:"model"`
	MaxTokens         int    `yaml:"max_tokens"`
	MaxParallelAgents int    `yaml:"max_parallel_agents"`
	DefaultMode       string `yaml:"mode"`
	SafeMode          bool   `yaml:"safe_mode"`
}

func DefaultConfig() Config {
	return Config{
		BaseURL:           "https://api.minimax.io/anthropic/v1/messages",
		Model:             "minimax-m2.1",
		MaxTokens:         2048,
		MaxParallelAgents: 50,
		DefaultMode:       "plan",
		SafeMode:          true,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Model == "" {
		cfg.Model = "minimax-m2.1"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.minimax.io/anthropic/v1/messages"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 2048
	}
	if cfg.MaxParallelAgents <= 0 {
		cfg.MaxParallelAgents = 50
	}
	if cfg.MaxParallelAgents > 900 {
		cfg.MaxParallelAgents = 900
	}
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = "plan"
	}
	return cfg, nil
}

func DefaultConfigPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "cli-agent", "config.yml")
}
