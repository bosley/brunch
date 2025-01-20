package brunch

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

type NodeTyppe string

type NttProvider interface {
	NewNett() RootNode
	ExtendFrom(NttNode) MessageCreator
	GetRoot(NttNode) RootNode
	GetHistory(NttNode) []map[string]string
	QueueImages([]string) error
}

const (
	NT_ROOT         NodeTyppe = "root"
	NT_MESSAGE_PAIR NodeTyppe = "message_pair"
)

type NttNode interface {
	Type() NodeTyppe
	Hash() string
	ToString() string
	History() []string
}

// Provider must create a function that the user can call to create a new message pair node
type MessageCreator func(userMessage string) (*MessagePairNode, error)

type Node struct {
	Type NodeTyppe `json:"type"`

	Parent   NttNode   `json:"parent,omitempty"`
	Children []NttNode `json:"children,omitempty"`
}

type RootNode struct {
	Node
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

func (r *RootNode) Type() NodeTyppe {
	return NT_ROOT
}

func (r *RootNode) Hash() string {
	hasher := sha256.New()
	hasher.Write([]byte(r.Provider + r.Model + r.Prompt + strconv.FormatFloat(r.Temperature, 'f', -1, 64) + strconv.Itoa(r.MaxTokens)))
	return hex.EncodeToString(hasher.Sum(nil))
}

type RootOpt struct {
	Provider    string
	Model       string
	Prompt      string
	Temperature float64
	MaxTokens   int
}

type MessagePairNode struct {
	Node
	Assistant *MessageData `json:"assistant"`
	User      *MessageData `json:"user"`
	Time      time.Time    `json:"time"`
}

func (m *MessagePairNode) Type() NodeTyppe {
	return NT_MESSAGE_PAIR
}

func (m *MessagePairNode) Hash() string {
	hasher := sha256.New()
	if m.Assistant == nil || m.User == nil {
		return ""
	}
	hasher.Write([]byte(m.Assistant.B64EncodedContent + m.User.B64EncodedContent + m.Time.Format(time.RFC3339)))
	return hex.EncodeToString(hasher.Sum(nil))
}

type MessageData struct {
	Role              string `json:"role"`
	B64EncodedContent string `json:"b64_encoded_content"`
}

func NewRootNode(opts RootOpt) *RootNode {
	root := &RootNode{
		Node:        Node{Type: NT_ROOT},
		Provider:    opts.Provider,
		Model:       opts.Model,
		Prompt:      opts.Prompt,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	}
	return root
}

func NewMessageData(role string, unencodedContent string) *MessageData {
	return &MessageData{
		Role:              role,
		B64EncodedContent: base64.StdEncoding.EncodeToString([]byte(unencodedContent)),
	}
}

func (m *MessageData) UnencodedContent() string {
	decoded, err := base64.StdEncoding.DecodeString(m.B64EncodedContent)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func (m *MessageData) updateContent(content string) {
	m.B64EncodedContent = base64.StdEncoding.EncodeToString([]byte(content))
}

func (m *Node) History() []string {
	messages := []MessageData{}

	if m.Parent != nil {
		messages = historyFromNode(m.Parent, messages)
	}

	if m.Type == NT_MESSAGE_PAIR {
		if mp, ok := interface{}(m).(*MessagePairNode); ok {
			if mp.User != nil {
				messages = append(messages, *mp.User)
			}
			if mp.Assistant != nil {
				messages = append(messages, *mp.Assistant)
			}
		}
	}

	result := make([]string, len(messages))
	for i, msg := range messages {
		result[i] = messageToString(&msg)
	}
	return result
}

func (m *Node) ToString() string {
	if m.Type == NT_MESSAGE_PAIR {
		if mp, ok := interface{}(m).(*MessagePairNode); ok {
			return fmt.Sprintf("User: %s\nAssistant: %s", mp.User.UnencodedContent(), mp.Assistant.UnencodedContent())
		}
	}
	return fmt.Sprintf("Node: %s", m.Type)
}

func historyFromNode(node NttNode, list []MessageData) []MessageData {
	if node == nil {
		return list
	}

	if node.Type() != NT_ROOT {
		if mp, ok := node.(*MessagePairNode); ok && mp.Parent != nil {
			list = historyFromNode(mp.Parent, list)
		}
	}

	if node.Type() == NT_MESSAGE_PAIR {
		if mp, ok := node.(*MessagePairNode); ok && mp.Assistant != nil && mp.User != nil {
			list = append(list, *mp.User, *mp.Assistant)
		}
	}
	return list
}
