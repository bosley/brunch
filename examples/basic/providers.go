package main

import (
	"fmt"
	"os"

	"github.com/bosley/brunch"
	"github.com/bosley/brunch/anthropic"
)

func clientFromSelectedProvider(config *Config) brunch.Provider {
	switch config.Providers[config.SelectedProvider].Name {
	case ProviderAnthropicChat:
		return anthropicChat(config)
	default:
		fmt.Println("Unknown provider")
		os.Exit(1)
	}
	return nil
}

func anthropicChat(config *Config) brunch.Provider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set ANTHROPIC_API_KEY environment variable")
		os.Exit(1)
	}
	client, err := anthropic.New(
		apiKey,
		config.Providers[config.SelectedProvider].SystemPrompt,
		config.Providers[config.SelectedProvider].Temperature,
		config.Providers[config.SelectedProvider].MaxTokens,
	)

	if err != nil {
		fmt.Printf("Failed to create Anthropic client: %v\n", err)
		os.Exit(1)
	}
	return anthropic.NewAnthropicProvider(client)
}
