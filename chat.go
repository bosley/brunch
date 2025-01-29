package brunch

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// The panel is an interface for the user of brunch to interact with our chat instance
// in a way that is easy to understand and use
type Conversation interface {

	// Print the entire tree of the conversation, which includes all branches
	PrintTree() string

	// Print the history of the conversation, on the current branch back to the root
	PrintHistory() string

	// Queue images to be sent to the provider
	QueueImages(paths []string) error

	// Snapshot the current state of the conversation
	Snapshot() (*Snapshot, error)

	// Get the artifacts from the current node (not the entire conversation)
	Artifacts() []Artifact

	// Attach a context to the conversation that the chat provider _may_ use
	// within the conversation that is ongoing
	CreateContext(ctx *ContextSettings) error

	// Attach an existing context to the conversation
	AttachContext(ctxName string) error

	// Goto a specific node in the conversation via hash (use PrintTree of History to see hashes)
	Goto(nodeHash string) error

	// Navigate to the parent node of the current node
	Parent() error

	// Navigate to the nth child of the current node
	Child(idx int) error

	// Navigate to the root node of the conversation
	Root() error

	// List the children of the current node
	ListChildren() []string

	// Check if the current node has a parent
	HasParent() bool

	// Toggle the chat on or off (soft disable)
	ToggleChat(enabled bool)

	// Get info about the current state of the chat
	Info() string

	// Get the current node of the conversation
	CurrentNode() Node

	// Submit a message to the chat provider
	SubmitMessage(message string) (string, error)

	// List the knowledge contexts that are attached to the conversation
	ListKnowledgeContexts() []string
}

// The snapshot is a hollistic snapshot of the current state of the chat
// It includes the provider name, the active branch, the contents of the conversation,
// and the contexts that are attached to the conversation. If a chat is saved with contexts
// then then all of the contextual configuration information MUST be available at the time of
// load. Snapshots save references to internal brunch resources on disk so they must
// be persisent and available at the time of load.
type Snapshot struct {
	ProviderName string   `json:"provider_name"`
	ActiveBranch string   `json:"active_branch"`
	Contents     []byte   `json:"contents"`
	Contexts     []string `json:"contexts"`
}

func (s *Snapshot) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func SnapshotFromJSON(data []byte) (*Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	return &snapshot, nil
}

type chatInstance struct {
	core         *Core
	provider     Provider
	root         RootNode
	currentNode  Node
	chatEnabled  bool
	queuedImages []string

	contexts map[string]*ContextSettings
}

func newChatInstance(provider Provider) *chatInstance {
	root := provider.NewConversationRoot()
	chat := &chatInstance{
		provider:     provider,
		root:         root,
		chatEnabled:  true,
		queuedImages: []string{},
		contexts:     map[string]*ContextSettings{},
	}
	chat.currentNode = &chat.root
	return chat
}

func newChatInstanceFromSnapshot(core *Core, snap *Snapshot) (*chatInstance, error) {
	root, err := unmarshalNode(snap.Contents)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	rootNode, ok := root.(*RootNode)
	if !ok {
		return nil, fmt.Errorf("snapshot does not contain a valid root node")
	}

	provider, exists := core.providers[snap.ProviderName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", snap.ProviderName)
	}

	chat := &chatInstance{
		core:         core,
		provider:     provider,
		root:         *rootNode,
		chatEnabled:  true,
		queuedImages: []string{},
		contexts:     map[string]*ContextSettings{},
	}
	chat.currentNode = &chat.root

	for _, ctxName := range snap.Contexts {
		ctx, exists := core.contexts[ctxName]
		if !exists {
			return nil, fmt.Errorf("context %s not found in available contexts", ctxName)
		}
		if err := chat.provider.AttachKnowledgeContext(*ctx); err != nil {
			return nil, fmt.Errorf("failed to attach context %s: %w", ctxName, err)
		}
		chat.contexts[ctxName] = ctx
	}

	slog.Debug("loaded snapshot", "num_contexts", len(chat.contexts))

	if snap.ActiveBranch != "" {
		nodeMap := MapTree(&chat.root)
		if node, exists := nodeMap[snap.ActiveBranch]; exists {
			chat.currentNode = node
			return chat, nil
		}
		for hash, node := range nodeMap {
			if strings.HasPrefix(hash, snap.ActiveBranch) {
				chat.currentNode = node
				return chat, nil
			}
		}
		return nil, fmt.Errorf("could not find active branch %s in snapshot", snap.ActiveBranch)
	}

	return chat, nil
}

// SubmitMessage sends a message to the provider and returns the response
func (c *chatInstance) SubmitMessage(message string) (string, error) {
	if !c.chatEnabled {
		return "", nil
	}

	if len(c.queuedImages) > 0 {
		c.provider.QueueImages(c.queuedImages)
		c.queuedImages = []string{}
	}

	creator := c.provider.ExtendFrom(c.currentNode)
	msgPair, err := creator(message)
	if err != nil {
		return "", err
	}

	c.currentNode = msgPair
	return msgPair.Assistant.UnencodedContent(), nil
}

func (c *chatInstance) PrintTree() string {
	return PrintTree(&c.root)
}

func (c *chatInstance) PrintHistory() string {
	result := c.currentNode.History()
	switch c.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mp, ok := c.currentNode.(*MessagePairNode); ok && mp.Parent != nil {
			if len(mp.User.Images) > 0 {
				result = append(result, messageToStringWithImages(mp.User, mp.User.Images))
			} else {
				result = append(result, messageToString(mp.User))
			}
			if len(mp.Assistant.Images) > 0 {
				result = append(result, messageToStringWithImages(mp.Assistant, mp.Assistant.Images))
			} else {
				result = append(result, messageToString(mp.Assistant))
			}
		}
	}
	return strings.Join(result, "\n")
}

func (c *chatInstance) QueueImages(paths []string) error {
	c.queuedImages = append(c.queuedImages, paths...)
	return nil
}

func (c *chatInstance) Snapshot() (*Snapshot, error) {
	b, e := marshalNode(&c.root)
	if e != nil {
		return nil, e
	}

	contexts := []string{}
	for _, ctx := range c.contexts {
		contexts = append(contexts, ctx.Name)
	}
	s := &Snapshot{
		ProviderName: c.provider.Settings().Host,
		ActiveBranch: c.currentNode.Hash(),
		Contents:     b,
		Contexts:     contexts,
	}
	slog.Debug("snapshot", "snapshot", s, "num_contexts", len(contexts))
	return s, nil
}

func (c *chatInstance) Goto(nodeHash string) error {
	nodeMap := MapTree(&c.root)
	if node, exists := nodeMap[nodeHash]; exists {
		c.currentNode = node
		return nil
	}
	return errors.New("node not found")
}

func (c *chatInstance) Parent() error {
	switch c.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mpn, ok := c.currentNode.(*MessagePairNode); ok && mpn.Parent != nil {
			c.currentNode = mpn.Parent
			return nil
		}
		return errors.New("no parent found")
	case NT_ROOT:
		return nil
	}
	return errors.New("invalid node type")
}

func (c *chatInstance) Child(idx int) error {
	switch c.currentNode.Type() {
	case NT_ROOT:
		if rn, ok := c.currentNode.(*RootNode); ok && idx < len(rn.Children) {
			c.currentNode = rn.Children[idx]
			return nil
		}
		return errors.New("index out of bounds")
	case NT_MESSAGE_PAIR:
		if mpn, ok := c.currentNode.(*MessagePairNode); ok && idx < len(mpn.Children) {
			c.currentNode = mpn.Children[idx]
			return nil
		}
		return errors.New("index out of bounds")
	}
	return errors.New("invalid node type")
}

func (c *chatInstance) Root() error {
	c.currentNode = &c.root
	return nil
}

func (c *chatInstance) HasParent() bool {
	switch c.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mpn, ok := c.currentNode.(*MessagePairNode); ok {
			return mpn.Parent != nil
		}
	}
	return false
}

func (c *chatInstance) ListChildren() []string {
	switch c.currentNode.Type() {
	case NT_ROOT:
		if rn, ok := c.currentNode.(*RootNode); ok {
			children := []string{}
			for _, child := range rn.Children {
				children = append(children, child.Hash())
			}
			return children
		}
	case NT_MESSAGE_PAIR:
		if mpn, ok := c.currentNode.(*MessagePairNode); ok {
			children := []string{}
			for _, child := range mpn.Children {
				children = append(children, child.Hash())
			}
			return children
		}
	}
	return []string{}
}

func (c *chatInstance) Info() string {
	return fmt.Sprintf("current node: %s", c.currentNode.Hash())
}

func (c *chatInstance) ToggleChat(enabled bool) {
	c.chatEnabled = enabled
}

func (c *chatInstance) CurrentNode() Node {
	return c.currentNode
}

func (c *chatInstance) Artifacts() []Artifact {
	switch c.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mpn, ok := c.currentNode.(*MessagePairNode); ok {
			artifacts, err := ParseArtifactsFrom(mpn.Assistant)
			if err != nil {
				fmt.Println("error parsing artifacts:", err)
				return []Artifact{}
			}
			return artifacts
		}
	}
	return []Artifact{}
}

func (c *chatInstance) CreateContext(ctx *ContextSettings) error {
	if err := c.provider.AttachKnowledgeContext(*ctx); err != nil {
		return err
	}

	if err := c.core.newContextFromAttached(ctx); err != nil {
		return err
	}
	c.contexts[ctx.Name] = ctx
	return nil
}

func (c *chatInstance) AttachContext(ctxName string) error {
	ctx, exists := c.core.contexts[ctxName]
	if !exists {
		return fmt.Errorf("context %s not found", ctxName)
	}

	if err := c.provider.AttachKnowledgeContext(*ctx); err != nil {
		return err
	}

	c.contexts[ctxName] = ctx
	return nil
}

func (c *chatInstance) ListKnowledgeContexts() []string {
	contexts := []string{}
	for _, ctx := range c.contexts {
		contexts = append(contexts, ctx.Name)
	}
	return contexts
}
