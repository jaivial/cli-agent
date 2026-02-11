package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL = "https://api.z.ai/api/paas/v4/chat/completions"
	DefaultModel   = "glm-4.7"
	ModelGLM5      = "glm-5"
)

var SupportedModels = []string{DefaultModel, ModelGLM5}

type Config struct {
	APIKey            string `yaml:"eai_api_key"`
	BaseURL           string `yaml:"base_url"`
	Model             string `yaml:"model"`
	MaxTokens         int    `yaml:"max_tokens"`
	MaxParallelAgents int    `yaml:"max_parallel_agents"`
	DefaultMode       string `yaml:"mode"`
	SafeMode          bool   `yaml:"safe_mode"`
	Installed         bool   `yaml:"installed"`

	// TUI / chat UX preferences.
	ChatVerbosity string `yaml:"chat_verbosity"` // compact|balanced|detailed
	ShowBanner    bool   `yaml:"show_banner"`
	EnableMouse   bool   `yaml:"enable_mouse"`
	UseAltScreen  bool   `yaml:"alt_screen"`
}

func NormalizeModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case DefaultModel:
		return DefaultModel
	case ModelGLM5:
		return ModelGLM5
	default:
		return DefaultModel
	}
}

func NormalizeBaseURL(baseURL string) string {
	url := strings.TrimSpace(baseURL)
	if url == "" {
		return DefaultBaseURL
	}
	if strings.EqualFold(url, "mock://") {
		return "mock://"
	}
	if strings.Contains(strings.ToLower(url), "api.z.ai") {
		return strings.TrimRight(url, "/")
	}
	// Enforce Z.AI as the only provider.
	return DefaultBaseURL
}

func decodeConfig(data []byte, cfg *Config) error {
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return err
	}

	// Backward-compat migration for old config key name.
	if strings.TrimSpace(cfg.APIKey) == "" {
		var legacy struct {
			LegacyAPIKey string `yaml:"api_key"`
		}
		_ = yaml.Unmarshal(data, &legacy)
		if strings.TrimSpace(legacy.LegacyAPIKey) != "" {
			cfg.APIKey = strings.TrimSpace(legacy.LegacyAPIKey)
		}
	}
	return nil
}

func DefaultConfig() Config {
	return Config{
		BaseURL:           DefaultBaseURL,
		Model:             DefaultModel,
		MaxTokens:         4096,
		MaxParallelAgents: 50,
		DefaultMode:       "plan",
		SafeMode:          true,
		Installed:         false,

		ChatVerbosity: "compact",
		ShowBanner:    false,
		EnableMouse:   true,
		UseAltScreen:  true,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	// Try binary directory first
	if execPath, err := os.Executable(); err == nil {
		binaryDir := filepath.Dir(execPath)
		binaryConfig := filepath.Join(binaryDir, "settings.json")
		if data, err := os.ReadFile(binaryConfig); err == nil {
			if err := decodeConfig(data, &cfg); err == nil {
				cfg.Model = NormalizeModel(cfg.Model)
				cfg.BaseURL = NormalizeBaseURL(cfg.BaseURL)
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
	if err := decodeConfig(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.Model = NormalizeModel(cfg.Model)
	cfg.BaseURL = NormalizeBaseURL(cfg.BaseURL)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
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
