package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed prompts
var prompts embed.FS

const (
	DefaultMaxTokens    = 4096
	DefaultTemperature  = 0.7
	DefaultSystemPrompt = "prompts/sp-think.xml"
	DefaultConfigName   = "branch.yaml"
)

type AvailableProviders string

const (
	ProviderAnthropicChat AvailableProviders = "anthropic-chat"
)

type SystemPromptSource string

type Provider struct {
	Name         AvailableProviders `yaml:"name"`
	MaxTokens    int                `yaml:"max_tokens"`
	Temperature  float64            `yaml:"temperature"`
	SystemPrompt string             `yaml:"system_prompt"` // overrides the default if set
}

type Config struct {
	SelectedProvider AvailableProviders `yaml:"selected_provider"`
	Providers        map[AvailableProviders]Provider
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

	// Create default config
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

	// Marshal config to YAML
	yamlData, err := yaml.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func LoadFromDir(dir string) (*Config, error) {
	configPath := filepath.Join(dir, DefaultConfigName)

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the config
	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("no providers configured in config file")
	}

	// Ensure each provider has required fields
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
