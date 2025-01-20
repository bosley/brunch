package brunch

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type NodeTyppe string

type Provider interface {
	NewNett() RootNode
	ExtendFrom(Node) MessageCreator
	GetRoot(Node) RootNode
	GetHistory(Node) []map[string]string
	QueueImages([]string) error
}

const (
	NT_ROOT         NodeTyppe = "root"
	NT_MESSAGE_PAIR NodeTyppe = "message_pair"
)

type Node interface {
	Type() NodeTyppe
	Hash() string
	ToString() string
	History() []string
}

// Provider must create a function that the user can call to create a new message pair node
type MessageCreator func(userMessage string) (*MessagePairNode, error)

type NodeImpl struct {
	Type NodeTyppe `json:"type"`

	Parent   Node   `json:"parent,omitempty"`
	Children []Node `json:"children,omitempty"`
}

type RootNode struct {
	NodeImpl
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
	NodeImpl
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
	Role              string   `json:"role"`
	B64EncodedContent string   `json:"b64_encoded_content"`
	Images            []string `json:"images,omitempty"`
}

func NewRootNode(opts RootOpt) *RootNode {
	root := &RootNode{
		NodeImpl:    NodeImpl{Type: NT_ROOT},
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

func (m *NodeImpl) History() []string {
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
		if msg.Images != nil {
			result[i] = messageToStringWithImages(&msg, msg.Images)
		} else {
			result[i] = messageToString(&msg)
		}
	}
	return result
}

func (m *NodeImpl) ToString() string {
	if m.Type == NT_MESSAGE_PAIR {
		if mp, ok := interface{}(m).(*MessagePairNode); ok {
			return fmt.Sprintf("User: %s\nAssistant: %s", mp.User.UnencodedContent(), mp.Assistant.UnencodedContent())
		}
	}
	return fmt.Sprintf("Node: %s", m.Type)
}

func historyFromNode(node Node, list []MessageData) []MessageData {
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

func MarshalNode(node Node) ([]byte, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot marshal nil node")
	}
	switch n := node.(type) {
	case *RootNode:
		return json.Marshal(n)
	case *MessagePairNode:
		return json.Marshal(n)
	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type())
	}
}

func UnmarshalNode(data []byte) (Node, error) {
	var base NodeImpl
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node type: %w", err)
	}

	switch base.Type {
	case NT_ROOT:
		var root RootNode
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("failed to unmarshal root node: %w", err)
		}
		return &root, nil
	case NT_MESSAGE_PAIR:
		var msg MessagePairNode
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message pair node: %w", err)
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unknown node type: %s", base.Type)
	}
}
