// Package config loads and persists the runtime configuration for Genesis.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config is the persisted application configuration.
type Config struct {
	APIKey            string  `json:"apiKey"`
	BaseURL           string  `json:"baseUrl"`
	Model             string  `json:"model"`
	Addr              string  `json:"addr"`
	DataDir           string  `json:"dataDir"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"maxTokens"`
	MaxSteps          int     `json:"maxSteps"`
	ToolChoice        string  `json:"toolChoice"`
	SystemPrompt      string  `json:"systemPrompt"`
	ParallelToolCalls bool    `json:"parallelToolCalls"`
	ReasoningEffort   string  `json:"reasoningEffort"`
	ChoicesEnabled    bool    `json:"choicesEnabled"`
	SpeechModel       string  `json:"speechModel"`
	SpeechVoice       string  `json:"speechVoice"`
	SpeechFormat      string  `json:"speechFormat"`
	SpeechSpeed       float64 `json:"speechSpeed"`
	AutoSpeak         bool    `json:"autoSpeak"`
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		BaseURL:           "https://openrouter.ai/api/v1",
		Model:             "openai/gpt-4o-mini",
		Addr:              "127.0.0.1:8080",
		DataDir:           ".",
		Temperature:       1.0,
		MaxTokens:         2048,
		MaxSteps:          6,
		ToolChoice:        "auto",
		ParallelToolCalls: true,
		ReasoningEffort:   "off",
		ChoicesEnabled:    true,
		SpeechModel:       "openai/gpt-4o-mini-tts",
		SpeechVoice:       "alloy",
		SpeechFormat:      "mp3",
		SpeechSpeed:       1.0,
		AutoSpeak:         false,
	}
}

func (c *Config) normalize() {
	d := Default()
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = d.BaseURL
	}
	if strings.TrimSpace(c.Model) == "" {
		c.Model = d.Model
	}
	if strings.TrimSpace(c.Addr) == "" {
		c.Addr = d.Addr
	}
	if strings.TrimSpace(c.DataDir) == "" {
		c.DataDir = d.DataDir
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = d.MaxTokens
	}
	if c.MaxSteps <= 0 {
		c.MaxSteps = d.MaxSteps
	}
	if strings.TrimSpace(c.ToolChoice) == "" {
		c.ToolChoice = d.ToolChoice
	}
	if strings.TrimSpace(c.ReasoningEffort) == "" {
		c.ReasoningEffort = d.ReasoningEffort
	}
	if strings.TrimSpace(c.SpeechFormat) == "" {
		c.SpeechFormat = d.SpeechFormat
	}
	if c.SpeechSpeed <= 0 {
		c.SpeechSpeed = d.SpeechSpeed
	}
}

// Store is a concurrency-safe, file-backed configuration holder.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// Load reads configuration from path (which may be empty). Environment
// variables override file values so deployments can stay file-free.
func Load(path string) (*Store, error) {
	cfg := Default()
	if strings.TrimSpace(path) != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, fmt.Errorf("parse config %s: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist):
			// first run; defaults win
		default:
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	applyEnv(&cfg)
	cfg.normalize()
	return &Store{path: path, cfg: cfg}, nil
}

func applyEnv(c *Config) {
	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("OPENROUTER_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("GENESIS_MODEL"); v != "" {
		c.Model = v
	}
	if v := os.Getenv("GENESIS_ADDR"); v != "" {
		c.Addr = v
	}
	if v := os.Getenv("GENESIS_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("GENESIS_REASONING_EFFORT"); v != "" {
		c.ReasoningEffort = v
	}
	if v := os.Getenv("GENESIS_SPEECH_MODEL"); v != "" {
		c.SpeechModel = v
	}
	if v := os.Getenv("GENESIS_SPEECH_VOICE"); v != "" {
		c.SpeechVoice = v
	}
}

// Get returns a copy of the current configuration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Path returns the backing file path (may be empty).
func (s *Store) Path() string { return s.path }

// Set replaces the configuration and persists it.
func (s *Store) Set(c Config) error {
	c.normalize()
	s.mu.Lock()
	s.cfg = c
	s.mu.Unlock()
	return s.Save()
}

// Update mutates the configuration in place and persists it.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	fn(&s.cfg)
	s.cfg.normalize()
	s.mu.Unlock()
	return s.Save()
}

// Save writes the configuration to disk.
func (s *Store) Save() error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(b, '\n'), 0o600)
}

// Public is the redacted configuration sent to the browser.
func (s *Store) Public() map[string]any {
	c := s.Get()
	return map[string]any{
		"hasKey":            strings.TrimSpace(c.APIKey) != "",
		"baseUrl":           c.BaseURL,
		"model":             c.Model,
		"temperature":       c.Temperature,
		"maxTokens":         c.MaxTokens,
		"maxSteps":          c.MaxSteps,
		"toolChoice":        c.ToolChoice,
		"systemPrompt":      c.SystemPrompt,
		"parallelToolCalls": c.ParallelToolCalls,
		"reasoningEffort":   c.ReasoningEffort,
		"choicesEnabled":    c.ChoicesEnabled,
		"speechModel":       c.SpeechModel,
		"speechVoice":       c.SpeechVoice,
		"speechFormat":      c.SpeechFormat,
		"speechSpeed":       c.SpeechSpeed,
		"autoSpeak":         c.AutoSpeak,
	}
}

// LoadDotEnv loads simple KEY=VALUE pairs from path into the process
// environment without overwriting variables that are already set. It is
// intentionally dependency-free.
func LoadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, "\"'")
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
