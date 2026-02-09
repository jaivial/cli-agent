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
	Installed         bool   `yaml:"installed"`
}

func DefaultConfig() Config {
	return Config{
		BaseURL:           "https://api.minimax.io/anthropic/v1/messages",
		Model:             "minimax-m2.1",
		MaxTokens:         4096,
		MaxParallelAgents: 50,
		DefaultMode:       "plan",
		SafeMode:          true,
		Installed:         false,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	// Try binary directory first
	if execPath, err := os.Executable(); err == nil {
		binaryDir := filepath.Dir(execPath)
		binaryConfig := filepath.Join(binaryDir, "settings.json")
		if data, err := os.ReadFile(binaryConfig); err == nil {
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				cfg.Installed = true
				return cfg, nil
			}
		}
	}

	// Fall back to provided path
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

func SaveConfig(cfg Config, path string) error {
	// Try binary directory first
	if execPath, err := os.Executable(); err == nil {
		binaryDir := filepath.Dir(execPath)
		binaryConfig := filepath.Join(binaryDir, "settings.json")
		cfg.Installed = true
		data, _ := yaml.Marshal(cfg)
		if err := os.WriteFile(binaryConfig, data, 0644); err == nil {
			return nil
		}
	}

	// Fall back to provided path
	if path == "" {
		return errors.New("no path provided for config")
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func DefaultConfigPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "cli-agent", "config.yml")
}

func GetBinaryConfigPath() string {
	if execPath, err := os.Executable(); err == nil {
		binaryDir := filepath.Dir(execPath)
		return filepath.Join(binaryDir, "settings.json")
	}
	return ""
}
