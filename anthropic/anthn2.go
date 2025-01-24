package anthropic

import (
	"fmt"
	"os"

	"github.com/bosley/brunch"
)

const (
	DefaultTemperature = 0.7
	DefaultMaxTokens   = 4000
	AbsoluteMaxTokens  = 4096
)

type AnthropicProvider struct {
	client        *Client
	pendingImages []string
}

var _ brunch.Provider = (*AnthropicProvider)(nil)

func InitialAnthropicProvider() brunch.Provider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set ANTHROPIC_API_KEY environment variable")
		os.Exit(1)
	}
	client, err := New(
		apiKey,
		"",
		0.7,
		4000,
	)
	if err != nil {
		fmt.Printf("Failed to create Anthropic client: %v\n", err)
		os.Exit(1)
	}
	return NewAnthropicProvider(client)
}

func (ap *AnthropicProvider) MaxTokens() int {
	return ap.client.maxTokens
}

func NewAnthropicProvider(client *Client) *AnthropicProvider {
	return &AnthropicProvider{
		client:        client,
		pendingImages: []string{},
	}
}

func (ap *AnthropicProvider) NewConversationRoot() brunch.RootNode {
	return *brunch.NewRootNode(brunch.RootOpt{
		Provider:    "anthropic",
		Model:       ap.client.model,
		Prompt:      ap.client.systemPrompt,
		Temperature: ap.client.temperature,
		MaxTokens:   ap.client.maxTokens,
	})
}

func (ap *AnthropicProvider) ExtendFrom(node brunch.Node) brunch.MessageCreator {
	msgPair := brunch.NewMessagePairNode(node)

	switch parent := node.(type) {
	case *brunch.RootNode:
		parent.AddChild(msgPair)
	case *brunch.MessagePairNode:
		parent.AddChild(msgPair)
	}

	return func(userMessage string) (*brunch.MessagePairNode, error) {
		ap.client.Reset()
		localClient := ap.client.Copy()
		history := ap.GetHistory(node)
		for _, msg := range history {
			localClient.conversations = append(localClient.conversations, Message{
				Role:    msg["role"],
				Content: msg["content"],
			})
		}

		var resp string
		var err error
		var usedImages []string

		if len(ap.pendingImages) > 0 {
			usedImages = ap.pendingImages
			resp, err = localClient.AskWithImage(userMessage, ap.pendingImages)
		} else {
			resp, err = localClient.Ask(userMessage)
		}

		if err != nil {
			return nil, err
		}
		msgPair.User = brunch.NewMessageData("user", userMessage)
		msgPair.Assistant = brunch.NewMessageData("assistant", resp)

		if len(usedImages) > 0 {
			msgPair.User.Images = usedImages
		}
		ap.pendingImages = []string{}
		return msgPair, nil
	}
}

func (ap *AnthropicProvider) GetRoot(node brunch.Node) brunch.RootNode {
	current := node
	for {
		if current.Type() == brunch.NT_ROOT {
			if root, ok := current.(*brunch.RootNode); ok {
				return *root
			}
		}

		if msgPair, ok := current.(*brunch.MessagePairNode); ok {
			if msgPair.Parent != nil {
				current = msgPair.Parent
				continue
			}
		}

		return *brunch.NewRootNode(brunch.RootOpt{
			Provider: "anthropic",
		})
	}
}

func (ap *AnthropicProvider) GetHistory(node brunch.Node) []map[string]string {
	var history []map[string]string
	current := node
	for {
		if msgPair, ok := current.(*brunch.MessagePairNode); ok {
			if msgPair.Assistant != nil && msgPair.User != nil {
				history = append([]map[string]string{
					{
						"role":    msgPair.Assistant.Role,
						"content": msgPair.Assistant.UnencodedContent(),
					},
					{
						"role":    msgPair.User.Role,
						"content": msgPair.User.UnencodedContent(),
					},
				}, history...)
			}

			if msgPair.Parent != nil {
				current = msgPair.Parent
				continue
			}
		}
		break
	}
	return history
}

func (ap *AnthropicProvider) QueueImages(paths []string) error {
	ap.pendingImages = append(ap.pendingImages, paths...)
	return nil
}

func (ap *AnthropicProvider) Settings() brunch.ProviderSettings {
	return brunch.ProviderSettings{
		BaseUrl:      ap.client.apiEndpoint,
		MaxTokens:    ap.client.maxTokens,
		Temperature:  ap.client.temperature,
		SystemPrompt: ap.client.systemPrompt,
		Name:         "anthropic",
	}
}

func (ap *AnthropicProvider) CloneWithSettings(settings brunch.ProviderSettings) brunch.Provider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set ANTHROPIC_API_KEY environment variable")
		os.Exit(1)
	}
	client, err := New(
		apiKey,
		settings.SystemPrompt,
		settings.Temperature,
		settings.MaxTokens,
	)
	client.apiEndpoint = settings.BaseUrl
	if err != nil {
		fmt.Printf("Failed to create Anthropic client: %v\n", err)
		os.Exit(1)
	}
	return NewAnthropicProvider(client)
}
