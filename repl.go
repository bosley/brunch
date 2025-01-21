package brunch

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
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
type NttReplPanel interface {
	PrintTree() string
	PrintHistory() string
	GetRoutes() map[string]string
	TraverseToRoute(route string) error
	QueueImages(paths []string) error
	MapTree() map[string]Node
	Snapshot() ([]byte, error)
}

// Called when a command is entered
// If error is returned, it will be displayed to the user
// and the command will not be entered, and the message will
// not be added to the tree
type CommandHandler func(panel NttReplPanel, nodeHash, line string) error

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

func (r *Repl) GetRoutes() map[string]string {
	// Each node has a parent, and may have a liost of children.
	// we want to navigate
	routes := map[string]string{}
	if r.currentNode == nil {
		return map[string]string{}
	}
	switch r.currentNode.Type() {
	case NT_MESSAGE_PAIR:
		if mp, ok := r.currentNode.(*MessagePairNode); ok && mp.Parent != nil {
			routes["parent"] = "p:" + mp.Parent.Hash()
			for i, child := range mp.Children {
				routes[fmt.Sprintf("child-%d", i)] = "c:" + child.Hash()
			}
		}
	case NT_ROOT:
		if root, ok := r.currentNode.(*RootNode); ok {
			for i, child := range root.Children {
				routes[fmt.Sprintf("child-%d", i)] = "c:" + child.Hash()
			}
		}
	}
	return routes
}

func (r *Repl) TraverseToRoute(route string) error {
	parts := strings.Split(route, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid route: %s", route)
	}
	switch parts[0] {
	case "p":
		if r.currentNode.Type() == NT_MESSAGE_PAIR {
			if mp, ok := r.currentNode.(*MessagePairNode); ok && mp.Parent != nil {
				r.currentNode = mp.Parent
			}
		}
	case "c":
		if r.currentNode.Type() == NT_MESSAGE_PAIR {
			if mp, ok := r.currentNode.(*MessagePairNode); ok {
				index, err := strconv.Atoi(parts[1])
				if err != nil {
					return fmt.Errorf("invalid child index: %s", parts[1])
				}
				if index < 0 || index >= len(mp.Children) {
					return fmt.Errorf("child index out of bounds: %d", index)
				}
				r.currentNode = mp.Children[index]
			}
		}
	}
	return nil
}

func (r *Repl) QueueImages(paths []string) error {
	r.enqueueImages = append(r.enqueueImages, paths...)
	return nil
}

func (r *Repl) MapTree() map[string]Node {
	return MapTree(r.currentNode)
}

func (r *Repl) Snapshot() ([]byte, error) {
	return marshalNode(&r.root)
}
