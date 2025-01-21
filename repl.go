package brunch

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Called before user message is sent to the provider
// Passes string as pointer so that it can be modified
// if needed, err will cancel the request
type PreHook func(query *string) error

// Called after the provider has responded
// Passes string as pointer so that it can be modified
// if needed, err will cancel the request, and the response
// will not be added to the tree
type PostHook func(response *string) error

const (
	DefaultCommandKey uint8 = '\\'
)

// A "Control panel" handed to the user that called on the repl
// to allow them to change how they interact with the message set
// on the actual human interface
type Panel interface {
	PrintTree() string
	PrintHistory() string
	QueueImages(paths []string) error
	Snapshot() (*Snapshot, error)
}

// Called when a command is entered
// If error is returned, it will be displayed to the user
// and the command will not be entered, and the message will
// not be added to the tree
type CommandHandler func(panel Panel, nodeHash, line string) error

// CommandOpts is the set of commands that will be available to the user, supplied by
// program external to this library. We supply this so user can change the repl key trigger
// and what actual commands are available. When the command is entered, the handler is called
// with the panel, the current node hash, and the line entered by the user
type CommandOpts struct {
	KeyOn   uint8
	Handler CommandHandler
}

// The options for the repl
// Provider is the provider that will be used to create the nett (anthropic, openai, etc) - Bring your own
// PreHook is a function that will be called before the user's message is sent to the provider
// PostHook is a function that will be called after the provider has responded
// Commands is the set of commands that will be available to the user
// InterruptHandler is a function that will be called when the user interrupts the repl
// CompletionHandler is a function that will be called when the repl is complete from some other source
type ReplOpts struct {
	Provider          Provider
	PreHook           PreHook
	PostHook          PostHook
	Commands          CommandOpts
	InterruptHandler  func(Node)
	CompletionHandler func(Node)
}

// The main struct that holds the state of the repl
type Repl struct {
	provider          Provider
	preHook           PreHook
	postHook          PostHook
	commands          CommandOpts
	interruptHandler  func(Node)
	completionHandler func(Node)

	root        RootNode
	currentNode Node

	done chan bool

	enqueueImages []string
}

// Obviously to create a repl..
func NewRepl(opts ReplOpts) *Repl {
	return &Repl{
		provider:          opts.Provider,
		preHook:           opts.PreHook,
		postHook:          opts.PostHook,
		commands:          opts.Commands,
		interruptHandler:  opts.InterruptHandler,
		completionHandler: opts.CompletionHandler,
	}
}

func NewReplFromSnapshot(opts ReplOpts, snap *Snapshot) (*Repl, error) {
	repl := NewRepl(opts)

	// Unmarshal the snapshot
	root, err := unmarshalNode(snap.Contents)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	// Convert to RootNode
	if rootNode, ok := root.(*RootNode); ok {
		repl.root = *rootNode
		repl.currentNode = &repl.root
	} else {
		return nil, fmt.Errorf("snapshot does not contain a valid root node")
	}

	// Find and set the last active branch node
	if snap.ActiveBranch != "" {
		nodeMap := MapTree(&repl.root)
		// First try exact match
		if node, exists := nodeMap[snap.ActiveBranch]; exists {
			repl.currentNode = node
		} else {
			// Try prefix/suffix match for short/full hashes
			found := false
			for hash, node := range nodeMap {

				fmt.Println("hash:", hash)
				fmt.Println("lastActiveBranch:", snap.ActiveBranch)
				if strings.HasPrefix(hash, snap.ActiveBranch) || hash == snap.ActiveBranch {
					fmt.Println("Found match:", hash)
					repl.currentNode = node
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Warning: Last active branch %s not found in snapshot, starting at root\n", snap.ActiveBranch)
			}
		}
	}

	return repl, nil
}

func (r *Repl) Complete() {
	r.done <- true
}

// Run the repl - blocking until the user interrupts or the repl is marked "Complete()"
func (r *Repl) Run() {
	r.root = r.provider.NewConversationRoot()
	r.currentNode = &r.root

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	r.done = make(chan bool)

	shortHash := func() string {
		if r.currentNode == nil {
			return ""
		}
		return r.currentNode.Hash()[:8]
	}

	prompt := func() string {
		return fmt.Sprintf("\n%s> ", shortHash())
	}

	// Start chat loop in goroutine
	go func() {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Chat started. Press Ctrl+C to exit and view conversation tree.")
		fmt.Println("Enter your messages (press Enter twice to send):")

		for {
			var lines []string
			fmt.Print(prompt())

			// Read until double Enter
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					fmt.Printf("Error reading input: %v\n", err)
					r.done <- true
					return
				}

				line = strings.TrimSpace(line)
				if line == "" && len(lines) > 0 {
					break
				}
				if line != "" {
					if r.commands.KeyOn != 0 && strings.HasPrefix(line, string(r.commands.KeyOn)) {
						if err := r.commands.Handler(r, shortHash(), line); err != nil {
							fmt.Println("Command failed:", err)
							continue
						}
						fmt.Print(prompt())
					} else {
						lines = append(lines, line)
					}
				}
			}

			question := strings.Join(lines, "\n")

			if r.preHook != nil {
				err := r.preHook(&question)
				if err != nil {
					fmt.Println("Failed to run preHook", err)
					continue
				}
			}

			if len(r.enqueueImages) > 0 {
				r.provider.QueueImages(r.enqueueImages)
				r.enqueueImages = []string{}
			}

			creator := r.provider.ExtendFrom(r.currentNode)
			msgPair, err := creator(question)
			if err != nil {
				fmt.Println("Failed to create message pair node", err)
				continue
			}

			if r.postHook != nil {
				content := msgPair.Assistant.UnencodedContent()
				err := r.postHook(&content)
				if err != nil {
					fmt.Println("Failed to run postHook", err)
					continue
				}
				if content == "" {
					fmt.Println("PostHook returned empty content, skipping update")
					continue
				}
				msgPair.Assistant.updateContent(content)
			}

			r.currentNode = msgPair
		}
	}()

	select {
	case <-sigChan:
		if r.interruptHandler != nil {
			r.interruptHandler(&r.root)
		}
	case <-r.done:
		if r.completionHandler != nil {
			r.completionHandler(&r.root)
		}
	}
}

func (r *Repl) PrintTree() string {
	return PrintTree(&r.root)
}

func (r *Repl) PrintHistory() string {
	result := r.currentNode.History()
	switch r.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mp, ok := r.currentNode.(*MessagePairNode); ok && mp.Parent != nil {
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

func (r *Repl) QueueImages(paths []string) error {
	r.enqueueImages = append(r.enqueueImages, paths...)
	return nil
}

func (r *Repl) Snapshot() (*Snapshot, error) {
	b, e := marshalNode(&r.root)
	if e != nil {
		return nil, e
	}
	s := &Snapshot{
		ActiveBranch: r.currentNode.Hash(),
		Contents:     b,
	}
	return s, nil
}
