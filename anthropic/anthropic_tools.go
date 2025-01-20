package anthropic

// TODO: Work this into brunch so they can define callbacks and tools and whatnot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type ToolsClient struct {
	*Client
	toolCalls []ToolCall
}

type ToolCall struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`         // b64 user input
	Response  string    `json:"response"`        // b64 LLM response
	Tool      string    `json:"tool"`            // tool name
	Schema    string    `json:"schema"`          // b64 tool schema
	Error     string    `json:"error,omitempty"` // error message if failed
	Success   bool      `json:"success"`         // whether the call succeeded
	Timestamp time.Time `json:"timestamp"`
}

func NewTools(apiKey, systemPrompt string, temperature float64, maxTokens int) (*ToolsClient, error) {
	baseClient, err := New(apiKey, systemPrompt, temperature, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to create base client: %w", err)
	}

	return &ToolsClient{
		Client:    baseClient,
		toolCalls: make([]ToolCall, 0),
	}, nil
}

func (tc *ToolsClient) CallTool(input string, toolSchema string) (string, error) {
	// Base64 encode the raw user input
	encodedInput := base64.StdEncoding.EncodeToString([]byte(input))

	// Make the tool call using raw schema
	response, err := tc.Ask(fmt.Sprintf("%s\n\n%s", toolSchema, input))

	// Encode schema for storage
	encodedSchema := base64.StdEncoding.EncodeToString([]byte(toolSchema))

	// Create tool call record - store both success and failure
	toolCall := ToolCall{
		Role:      "user",
		Content:   encodedInput,
		Tool:      "tool",
		Schema:    encodedSchema,
		Success:   err == nil,
		Timestamp: time.Now(),
	}

	if err != nil {
		toolCall.Error = err.Error()
		toolCall.Response = ""
	} else {
		toolCall.Response = base64.StdEncoding.EncodeToString([]byte(response))
	}

	// Store the tool call
	tc.toolCalls = append(tc.toolCalls, toolCall)

	// Return original error
	if err != nil {
		return "", err
	}
	return response, nil
}

func (tc *ToolsClient) ExportToolCalls() ([]byte, error) {
	export := struct {
		Model      string     `json:"model"`
		ToolCalls  []ToolCall `json:"tool_calls"`
		ExportedAt time.Time  `json:"exported_at"`
	}{
		Model:      tc.model,
		ToolCalls:  tc.toolCalls,
		ExportedAt: time.Now(),
	}

	return json.Marshal(export)
}
