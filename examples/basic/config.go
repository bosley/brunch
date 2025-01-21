package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed prompts
var prompts embed.FS

const (
	DefaultMaxTokens    = 4096
	DefaultTemperature  = 0.7
	DefaultSystemPrompt = "prompts/sp-think.xml"
	DefaultConfigName   = "brunch.json"
)

type AvailableProviders string

const (
	ProviderAnthropicChat AvailableProviders = "anthropic-chat"
)

type SystemPromptSource string

type Provider struct {
	Name         AvailableProviders `json:"name"`
	MaxTokens    int                `json:"max_tokens"`
	Temperature  float64            `json:"temperature"`
	SystemPrompt string             `json:"system_prompt"` // overrides the default if set
}

type Config struct {
	SelectedProvider AvailableProviders              `json:"selected_provider"`
	Providers        map[AvailableProviders]Provider `json:"providers"`
	History          json.RawMessage                 `json:"history,omitempty"`
	LastActiveBranch string                          `json:"last_active_branch,omitempty"`
}

func loadPrompt(path string) (string, error) {
	content, err := prompts.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func InitDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	configPath := filepath.Join(dir, DefaultConfigName)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("directory already initialized")
	}

	rad, err := loadPrompt(DefaultSystemPrompt)
	if err != nil {
		fmt.Println("Failed to load system prompt:", err)
		os.Exit(1)
	}

	defaultConfig := Config{
		SelectedProvider: ProviderAnthropicChat,
		Providers: map[AvailableProviders]Provider{
			ProviderAnthropicChat: {
				Name:         ProviderAnthropicChat,
				MaxTokens:    DefaultMaxTokens,
				Temperature:  DefaultTemperature,
				SystemPrompt: base64.StdEncoding.EncodeToString([]byte(rad)),
			},
		},
	}

	jsonData, err := json.MarshalIndent(defaultConfig, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func LoadFromDir(dir string) (*Config, error) {
	configPath := filepath.Join(dir, DefaultConfigName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("no providers configured in config file")
	}

	for name, provider := range config.Providers {
		if provider.Name != name {
			return nil, fmt.Errorf("provider name mismatch: %s != %s", provider.Name, name)
		}
		if provider.MaxTokens <= 0 {
			return nil, fmt.Errorf("invalid max_tokens for provider %s: must be > 0", name)
		}
		if provider.Temperature < 0 || provider.Temperature > 1 {
			return nil, fmt.Errorf("invalid temperature for provider %s: must be between 0 and 1", name)
		}
	}

	return &config, nil
}
