package anthropic

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type VisionClient struct {
	*Client
	visionCalls   []VisionCall
	conversations []VisionConversation
}

type VisionCall struct {
	Content   []MessagePart `json:"content"`
	Question  string        `json:"question"`
	Response  string        `json:"response"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

type VisionConversation struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`  // base64 encoded JSON of MessagePart array
	Response  string    `json:"response"` // base64 encoded response
	Timestamp time.Time `json:"timestamp"`
}

type ImageContent struct {
	Type      string `json:"type"`
	Source    Source `json:"source"`
	MediaType string `json:"media_type"`
}

type Source struct {
	Type      string `json:"type"`
	Data      string `json:"data,omitempty"`
	Path      string `json:"path,omitempty"`
	URL       string `json:"url,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

type VisionRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []VisionMessage `json:"messages"`
}

type VisionMessage struct {
	Role    string        `json:"role"`
	Content []MessagePart `json:"content"`
}

type MessagePart struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Source *struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
	} `json:"source,omitempty"`
}

type ClaudeVisionResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// NewVision creates a new VisionClient instance
func NewVision(apiKey, systemPrompt string, temperature float64, maxTokens int) (*VisionClient, error) {
	baseClient, err := New(apiKey, systemPrompt, temperature, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to create base client: %w", err)
	}

	return &VisionClient{
		Client:        baseClient,
		visionCalls:   make([]VisionCall, 0),
		conversations: make([]VisionConversation, 0),
	}, nil
}

// AskWithImage sends a question with one or more images to Claude
func (vc *VisionClient) AskWithImage(question string, imagePaths []string) (string, error) {
	content := make([]MessagePart, 0, len(imagePaths)+1)

	// Add images first
	for _, path := range imagePaths {
		imageData, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read image %s: %w", path, err)
		}

		mediaType := "image/jpeg" // default
		switch filepath.Ext(path) {
		case ".png":
			mediaType = "image/png"
		case ".jpeg", ".jpg":
			mediaType = "image/jpeg"
		case ".gif":
			mediaType = "image/gif"
		case ".webp":
			mediaType = "image/webp"
		}

		encoded := base64.StdEncoding.EncodeToString(imageData)

		content = append(content, MessagePart{
			Type: "image",
			Source: &struct {
				Type      string `json:"type"`
				MediaType string `json:"media_type"`
				Data      string `json:"data"`
			}{
				Type:      "base64",
				MediaType: mediaType,
				Data:      encoded,
			},
		})
	}

	// Add the question text
	content = append(content, MessagePart{
		Type: "text",
		Text: question,
	})

	// Encode the content for storage
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return "", fmt.Errorf("failed to marshal content: %w", err)
	}
	encodedContent := base64.StdEncoding.EncodeToString(contentJSON)

	reqBody := VisionRequest{
		Model:     vc.model,
		MaxTokens: vc.maxTokens,
		Messages: []VisionMessage{
			{
				Role:    "user",
				Content: content,
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Debug("vision request payload", "body", string(jsonBody))

	req, err := http.NewRequest("POST", DefaultAPIEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", vc.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Record the vision call
	visionCall := VisionCall{
		Content:   content,
		Question:  question,
		Response:  string(body),
		Success:   resp.StatusCode == http.StatusOK,
		Timestamp: time.Now(),
	}

	vc.visionCalls = append(vc.visionCalls, visionCall)

	// After getting successful response, store the conversation
	conversation := VisionConversation{
		Role:      "user",
		Content:   encodedContent,
		Timestamp: time.Now(),
	}

	var claudeResp ClaudeVisionResponse
	if err := json.Unmarshal([]byte(body), &claudeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Store both user question and assistant response (both base64 encoded)
	vc.conversations = append(vc.conversations, conversation)
	vc.conversations = append(vc.conversations, VisionConversation{
		Role:      "assistant",
		Response:  base64.StdEncoding.EncodeToString(body),
		Timestamp: time.Now(),
	})

	return string(body), nil
}

// ExportVisionCalls exports the vision calls to JSON
func (vc *VisionClient) ExportVisionCalls() ([]byte, error) {
	export := struct {
		Model       string       `json:"model"`
		VisionCalls []VisionCall `json:"vision_calls"`
		ExportedAt  time.Time    `json:"exported_at"`
	}{
		Model:       vc.model,
		VisionCalls: vc.visionCalls,
		ExportedAt:  time.Now(),
	}

	return json.Marshal(export)
}

// SaveVisionCallsToFile saves vision calls to a JSON file
func SaveVisionCallsToFile(vc *VisionClient) error {
	exportData, err := vc.ExportVisionCalls()
	if err != nil {
		return fmt.Errorf("failed to export vision calls: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("exports/claude_vision_%s.json", timestamp)

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, exportData, "", "    "); err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	if err := os.MkdirAll("exports", 0755); err != nil {
		return fmt.Errorf("failed to create exports directory: %w", err)
	}

	if err := os.WriteFile(filename, prettyJSON.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	slog.Info("vision calls exported", "filename", filename)
	return nil
}

// ExportConversations exports the vision conversations to JSON
func (vc *VisionClient) ExportConversations() ([]byte, error) {
	export := struct {
		Model         string               `json:"model"`
		Conversations []VisionConversation `json:"conversations"`
		ExportedAt    time.Time            `json:"exported_at"`
	}{
		Model:         vc.model,
		Conversations: vc.conversations,
		ExportedAt:    time.Now(),
	}

	return json.Marshal(export)
}

// SaveVisionConversationsToFile saves vision conversations to a JSON file
func SaveVisionConversationsToFile(vc *VisionClient) error {
	exportData, err := vc.ExportConversations()
	if err != nil {
		return fmt.Errorf("failed to export conversations: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("exports/claude_vision_conversation_%s.json", timestamp)

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, exportData, "", "    "); err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	if err := os.MkdirAll("exports", 0755); err != nil {
		return fmt.Errorf("failed to create exports directory: %w", err)
	}

	if err := os.WriteFile(filename, prettyJSON.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	slog.Info("vision conversations exported", "filename", filename)
	return nil
}
