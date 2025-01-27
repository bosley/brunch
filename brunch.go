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

type ProviderSettings struct {
	Name         string  `json:"name"`
	Host         string  `json:"host"`
	BaseUrl      string  `json:"base_url"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	SystemPrompt string  `json:"system_prompt"`
}

// A provider is an abstraction of some (presumably LLM) message generation service
// though it could be anything that generates messages i guess
type Provider interface {

	// NewConversationRoot creates a new root node for a new conversation
	// from which all messages will be derived
	NewConversationRoot() RootNode

	// ExtendFrom takes a node and returns a function that can be used to create a new message pair node
	// This means that this is the function we call in order to get a function to send a message,
	// and then receive a response
	ExtendFrom(Node) MessageCreator

	// GetRoot takes a node and returns the root node
	GetRoot(Node) RootNode

	// GetHistory takes a node and returns the history of the conversation
	GetHistory(Node) []map[string]string

	// QueueImages takes a list of image urls and queues them for attachment to the next message
	// In this way we can ask about images in the conversation and do analysis on them
	// If the provider doesn't support images, this should return an error
	QueueImages([]string) error

	// Settings returns the settings for the provider
	Settings() ProviderSettings

	// CloneWithSettings returns a new provider with the given settings
	// This is so we can derive providers from existing providers at runtime
	// and have them be available to the user
	CloneWithSettings(ProviderSettings) Provider

	// AttachKnowledgeContext attaches a knowledge context to the provider
	// A knowledge context could be a directory, a database, a web page, etc.
	// HOW the knowledge is incorperated into the conversation is up to the provider
	// and if the provider doesn't support knowledge contexts, this should return an error
	AttachKnowledgeContext(ContextSettings) error
}

// A context type is a type of knowledge that can be attached to a conversation
// This could be a directory, a database, a web page, etc.
// HOW the knowledge is incorperated into the conversation is up to the provider
// and if the provider doesn't support knowledge contexts, this should return an error
type ContextType string

const (
	ContextTypeDirectory ContextType = "directory"
	ContextTypeDatabase  ContextType = "database"
	ContextTypeWeb       ContextType = "web"
)

type ContextSettings struct {
	Name  string      `json:"name"`
	Type  ContextType `json:"type"`
	Value string      `json:"value"`
}

const (
	NT_ROOT         NodeTyppe = "root"
	NT_MESSAGE_PAIR NodeTyppe = "message_pair"
)

// A Node is either a root, or a PAIR of messages (user and provider)
// We seperate into pairs of messages to constrain the conversation
// as a request->generation || failure
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
	hasher.Write([]byte(m.Assistant.UnencodedContent() + m.User.UnencodedContent() + m.Time.Format(time.RFC3339)))
	return hex.EncodeToString(hasher.Sum(nil))
}

type MessageData struct {
	Role              string   `json:"role"`
	B64EncodedContent string   `json:"-"`
	RawContent        string   `json:"content"`
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

// NewMessageData creates a new message data object and ensures
// that the content is base64 encoded as when we save things we don't want messages
// to bonk our json, and it helps keep the data clean
func NewMessageData(role string, unencodedContent string) *MessageData {
	return &MessageData{
		Role:              role,
		B64EncodedContent: base64.StdEncoding.EncodeToString([]byte(unencodedContent)),
		RawContent:        unencodedContent,
	}
}

// UnencodedContent returns the raw content of the message
// if the message is not base64 encoded, it will return the base64 encoded content
func (m *MessageData) UnencodedContent() string {
	if m.RawContent != "" {
		return m.RawContent
	}
	decoded, err := base64.StdEncoding.DecodeString(m.B64EncodedContent)
	if err != nil {
		return m.B64EncodedContent
	}
	m.RawContent = string(decoded)
	return m.RawContent
}

func (m *MessageData) MarshalJSON() ([]byte, error) {
	type Alias MessageData
	return json.Marshal(&struct {
		Content string `json:"content"`
		*Alias
	}{
		Content: m.UnencodedContent(),
		Alias:   (*Alias)(m),
	})
}

func (m *MessageData) UnmarshalJSON(data []byte) error {
	type Alias MessageData
	aux := &struct {
		Content string `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.RawContent = aux.Content
	m.B64EncodedContent = base64.StdEncoding.EncodeToString([]byte(aux.Content))
	return nil
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
