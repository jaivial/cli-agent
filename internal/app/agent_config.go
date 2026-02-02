package app

import (
	"os"
	"strconv"
	"time"
)

// AgentConfig holds configurable parameters for the agent
type AgentConfig struct {
	DefaultTimeout            time.Duration
	VMTimeout                 time.Duration
	MaxHTTPResponseSize       int64
	ContextSummarizeThreshold int
	MaxStallCount             int
	ToolCacheExpiry           time.Duration
	ProcessCleanupDelay       time.Duration
	ConvergenceCheckInterval  int
	MaxOutputBufferSize       int
	MaxRetries                int
	RetryDelay                time.Duration
}

// DefaultAgentConfig returns the default configuration
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		DefaultTimeout:            30 * time.Second,
		VMTimeout:                 300 * time.Second,
		MaxHTTPResponseSize:       1024 * 1024, // 1MB
		ContextSummarizeThreshold: 20000,       // ~5000 tokens
		MaxStallCount:             6,
		ToolCacheExpiry:           5 * time.Minute,
		ProcessCleanupDelay:       5 * time.Minute,
		ConvergenceCheckInterval:  3,
		MaxOutputBufferSize:       1024 * 1024, // 1MB
		MaxRetries:                3,
		RetryDelay:                500 * time.Millisecond,
	}
}

// AgentConfigFromEnv loads configuration from environment variables
func AgentConfigFromEnv() AgentConfig {
	cfg := DefaultAgentConfig()

	if v := os.Getenv("EAI_DEFAULT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DefaultTimeout = d
		}
	}

	if v := os.Getenv("EAI_VM_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.VMTimeout = d
		}
	}

	if v := os.Getenv("EAI_MAX_HTTP_RESPONSE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxHTTPResponseSize = n
		}
	}

	if v := os.Getenv("EAI_CONTEXT_SUMMARIZE_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ContextSummarizeThreshold = n
		}
	}

	if v := os.Getenv("EAI_MAX_STALL_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxStallCount = n
		}
	}

	if v := os.Getenv("EAI_TOOL_CACHE_EXPIRY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ToolCacheExpiry = d
		}
	}

	if v := os.Getenv("EAI_PROCESS_CLEANUP_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ProcessCleanupDelay = d
		}
	}

	if v := os.Getenv("EAI_CONVERGENCE_CHECK_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ConvergenceCheckInterval = n
		}
	}

	if v := os.Getenv("EAI_MAX_OUTPUT_BUFFER_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxOutputBufferSize = n
		}
	}

	if v := os.Getenv("EAI_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxRetries = n
		}
	}

	if v := os.Getenv("EAI_RETRY_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RetryDelay = d
		}
	}

	return cfg
}
