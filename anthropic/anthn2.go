package anthropic

import (
	"time"

	"github.com/bosley/brunch"
)

type AnthropicProvider struct {
	client        *Client
	pendingImages []string
}

// NewAnthropicProvider creates a new brunch provider for Anthropic
func NewAnthropicProvider(client *Client) *AnthropicProvider {
	return &AnthropicProvider{
		client:        client,
		pendingImages: []string{},
	}
}

// NewNett creates a new root node with Anthropic-specific configuration
func (ap *AnthropicProvider) NewNett() brunch.RootNode {
	return *brunch.NewRootNode(brunch.RootOpt{
		Provider:    "anthropic",
		Model:       ap.client.model,
		Prompt:      ap.client.systemPrompt,
		Temperature: ap.client.temperature,
		MaxTokens:   ap.client.maxTokens,
	})
}

// ExtendFrom creates a new message pair node from the given node
func (ap *AnthropicProvider) ExtendFrom(node brunch.NttNode) brunch.MessageCreator {

	// Create a new message pair node
	msgPair := &brunch.MessagePairNode{
		Node: brunch.Node{
			Type:   brunch.NT_MESSAGE_PAIR,
			Parent: node,
		},
		Time: time.Now(),
	}

	switch parent := node.(type) {
	case *brunch.RootNode:
		parent.Children = append(parent.Children, msgPair)
	case *brunch.MessagePairNode:
		parent.Children = append(parent.Children, msgPair)
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

		if len(ap.pendingImages) > 0 {
			resp, err = localClient.AskWithImage(userMessage, ap.pendingImages)
			ap.pendingImages = []string{}
		} else {
			resp, err = localClient.Ask(userMessage)
		}
		ap.pendingImages = []string{}

		if err != nil {
			return nil, err
		}
		msgPair.User = brunch.NewMessageData("user", userMessage)
		msgPair.Assistant = brunch.NewMessageData("assistant", resp)
		return msgPair, nil
	}
}

// GetRoot traverses up the node tree to find the root node
func (ap *AnthropicProvider) GetRoot(node brunch.NttNode) brunch.RootNode {
	current := node
	for {
		if current.Type() == brunch.NT_ROOT {
			if root, ok := current.(*brunch.RootNode); ok {
				return *root
			}
		}

		// Type assert to access the Node struct
		if msgPair, ok := current.(*brunch.MessagePairNode); ok {
			if msgPair.Parent != nil {
				current = msgPair.Parent
				continue
			}
		}

		// If we can't find the root, return an empty root node
		return *brunch.NewRootNode(brunch.RootOpt{
			Provider: "anthropic",
		})
	}
}

// GetHistory returns the conversation history as a slice of message maps
func (ap *AnthropicProvider) GetHistory(node brunch.NttNode) []map[string]string {
	var history []map[string]string
	current := node

	// Traverse up the tree collecting messages
	for {
		if msgPair, ok := current.(*brunch.MessagePairNode); ok {
			if msgPair.Assistant != nil && msgPair.User != nil {
				// Add the message pair to history
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
