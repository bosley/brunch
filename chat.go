package brunch

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

type Panel interface {
	PrintTree() string
	PrintHistory() string
	QueueImages(paths []string) error
	Snapshot() (*Snapshot, error)

	Artifacts() []Artifact

	AttachContext(ctx *ContextSettings) error

	Goto(nodeHash string) error
	Parent() error
	Child(idx int) error
	Root() error

	ListChildren() []string
	HasParent() bool

	ToggleChat(enabled bool)
	Info() string
}

type ChatInstance struct {
	core         *Core
	provider     Provider
	root         RootNode
	currentNode  Node
	chatEnabled  bool
	queuedImages []string

	contexts map[string]*ContextSettings
}

func NewChatInstance(provider Provider) *ChatInstance {
	root := provider.NewConversationRoot()
	chat := &ChatInstance{
		provider:     provider,
		root:         root,
		chatEnabled:  true,
		queuedImages: []string{},
		contexts:     map[string]*ContextSettings{},
	}
	chat.currentNode = &chat.root
	return chat
}

func NewChatInstanceFromSnapshot(core *Core, snap *Snapshot) (*ChatInstance, error) {
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

	chat := &ChatInstance{
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
func (c *ChatInstance) SubmitMessage(message string) (string, error) {
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

	fmt.Println("attached contexts:", c.contexts)

	c.currentNode = msgPair
	return msgPair.Assistant.UnencodedContent(), nil
}

func (c *ChatInstance) PrintTree() string {
	return PrintTree(&c.root)
}

func (c *ChatInstance) PrintHistory() string {
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

func (c *ChatInstance) QueueImages(paths []string) error {
	c.queuedImages = append(c.queuedImages, paths...)
	return nil
}

func (c *ChatInstance) Snapshot() (*Snapshot, error) {
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

func (c *ChatInstance) Goto(nodeHash string) error {
	nodeMap := MapTree(&c.root)
	if node, exists := nodeMap[nodeHash]; exists {
		c.currentNode = node
		return nil
	}
	return errors.New("node not found")
}

func (c *ChatInstance) Parent() error {
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

func (c *ChatInstance) Child(idx int) error {
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

func (c *ChatInstance) Root() error {
	c.currentNode = &c.root
	return nil
}

func (c *ChatInstance) HasParent() bool {
	switch c.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mpn, ok := c.currentNode.(*MessagePairNode); ok {
			return mpn.Parent != nil
		}
	}
	return false
}

func (c *ChatInstance) ListChildren() []string {
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

func (c *ChatInstance) Info() string {
	return fmt.Sprintf("current node: %s", c.currentNode.Hash())
}

func (c *ChatInstance) ToggleChat(enabled bool) {
	c.chatEnabled = enabled
}

func (c *ChatInstance) CurrentNode() Node {
	return c.currentNode
}

func (c *ChatInstance) Artifacts() []Artifact {
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

func (c *ChatInstance) AttachContext(ctx *ContextSettings) error {
	if err := c.core.newContextFromAttached(ctx); err != nil {
		return err
	}
	c.contexts[ctx.Name] = ctx
	return nil
}
