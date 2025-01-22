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
	NewConversationRoot() RootNode
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
	ToMap() map[string]Node
}

// Provider must create a function that the user can call to create a new message pair node
type MessageCreator func(userMessage string) (*MessagePairNode, error)

type node struct {
	Type     NodeTyppe `json:"type"`
	Parent   Node      `json:"parent,omitempty"`
	Children []Node    `json:"children"`
}

func (n *node) AddChild(child Node) {
	if n.Children == nil {
		n.Children = make([]Node, 0, 1)
	}
	n.Children = append(n.Children, child)
}

func (n *node) ToMap() map[string]Node {
	r := make(map[string]Node)
	for _, child := range n.Children {
		r[child.Hash()] = child
	}
	return r
}

type RootNode struct {
	node
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
	node
	Assistant *MessageData `json:"assistant"`
	User      *MessageData `json:"user"`
	Time      time.Time    `json:"time"`
}

func NewMessagePairNode(parent Node) *MessagePairNode {
	return &MessagePairNode{
		node: node{
			Type:   NT_MESSAGE_PAIR,
			Parent: parent,
		},
		Time: time.Now(),
	}
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
		node:        node{Type: NT_ROOT},
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

func (m *node) History() []string {
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

func (m *node) ToString() string {
	if m.Type == NT_MESSAGE_PAIR {
		if mp, ok := interface{}(m).(*MessagePairNode); ok {
			return fmt.Sprintf("User: %s\nAssistant: %s", mp.User.UnencodedContent(), mp.Assistant.UnencodedContent())
		}
	} else if m.Type == NT_ROOT {
		if rn, ok := interface{}(m).(*RootNode); ok {
			return fmt.Sprintf("Root: %s", rn.Prompt)
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

func marshalNode(node Node) ([]byte, error) {
	type nodeDataRoot struct {
		Type        NodeTyppe `json:"type"`
		Provider    string    `json:"provider"`
		Model       string    `json:"model"`
		Prompt      string    `json:"prompt"`
		Temperature float64   `json:"temperature"`
		MaxTokens   int       `json:"max_tokens"`
	}

	type nodeDataMessagePair struct {
		Type      NodeTyppe    `json:"type"`
		Assistant *MessageData `json:"assistant"`
		User      *MessageData `json:"user"`
		Time      time.Time    `json:"time"`
	}

	type nodeWrapper struct {
		NodeData interface{}                `json:"node_data"`
		Children map[string]json.RawMessage `json:"children"`
	}

	wrapper := nodeWrapper{
		Children: make(map[string]json.RawMessage),
	}

	// Marshal children recursively
	for hash, child := range node.ToMap() {
		childData, err := marshalNode(child)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal child node: %w", err)
		}
		wrapper.Children[hash] = json.RawMessage(childData)
	}

	// Marshal node data based on type
	switch n := node.(type) {
	case *RootNode:
		wrapper.NodeData = nodeDataRoot{
			Type:        n.Type(),
			Provider:    n.Provider,
			Model:       n.Model,
			Prompt:      n.Prompt,
			Temperature: n.Temperature,
			MaxTokens:   n.MaxTokens,
		}
	case *MessagePairNode:
		wrapper.NodeData = nodeDataMessagePair{
			Type:      n.Type(),
			Assistant: n.Assistant,
			User:      n.User,
			Time:      n.Time,
		}
	default:
		return nil, fmt.Errorf("unknown node type: %T", node)
	}

	return json.Marshal(wrapper)
}

func unmarshalNode(data []byte) (Node, error) {
	var wrapper struct {
		NodeData json.RawMessage            `json:"node_data"`
		Children map[string]json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wrapper: %w", err)
	}

	// First, determine the node type
	var typeHolder struct {
		Type NodeTyppe `json:"type"`
	}
	if err := json.Unmarshal(wrapper.NodeData, &typeHolder); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node type: %w", err)
	}

	var result Node

	// Unmarshal based on node type
	switch typeHolder.Type {
	case NT_ROOT:
		var rootData struct {
			Type        NodeTyppe `json:"type"`
			Provider    string    `json:"provider"`
			Model       string    `json:"model"`
			Prompt      string    `json:"prompt"`
			Temperature float64   `json:"temperature"`
			MaxTokens   int       `json:"max_tokens"`
		}
		if err := json.Unmarshal(wrapper.NodeData, &rootData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal root node: %w", err)
		}
		result = NewRootNode(RootOpt{
			Provider:    rootData.Provider,
			Model:       rootData.Model,
			Prompt:      rootData.Prompt,
			Temperature: rootData.Temperature,
			MaxTokens:   rootData.MaxTokens,
		})

	case NT_MESSAGE_PAIR:
		var msgData struct {
			Type      NodeTyppe    `json:"type"`
			Assistant *MessageData `json:"assistant"`
			User      *MessageData `json:"user"`
			Time      time.Time    `json:"time"`
		}
		if err := json.Unmarshal(wrapper.NodeData, &msgData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message pair node: %w", err)
		}
		msgPair := NewMessagePairNode(nil) // Parent will be set when adding to children
		msgPair.Assistant = msgData.Assistant
		msgPair.User = msgData.User
		msgPair.Time = msgData.Time
		result = msgPair

	default:
		return nil, fmt.Errorf("unknown node type: %s", typeHolder.Type)
	}

	// Recursively unmarshal children
	if len(wrapper.Children) > 0 {
		children := make([]Node, 0, len(wrapper.Children))
		for _, childData := range wrapper.Children {
			child, err := unmarshalNode(childData)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal child node: %w", err)
			}
			children = append(children, child)
		}

		// Set parent-child relationships
		switch n := result.(type) {
		case *RootNode:
			n.Children = children
			for _, child := range children {
				if mp, ok := child.(*MessagePairNode); ok {
					mp.Parent = n
				}
			}
		case *MessagePairNode:
			n.Children = children
			for _, child := range children {
				if mp, ok := child.(*MessagePairNode); ok {
					mp.Parent = n
				}
			}
		}
	}

	return result, nil
}
