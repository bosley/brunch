package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bosley/brunch"
	"github.com/bosley/brunch/anthropic"
	"github.com/bosley/brunch/api"
)

type SessionState string

const (
	SSSelection   SessionState = "selection"   // User can CRUD their KV datastore or LOAD a convo from it
	SSInteraction SessionState = "interaction" // User is in an interactive conversation
)

type ChatConfig struct {
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	Prompt      string  `json:"system_prompt"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Messages    []struct {
		Role      string      `json:"role"`
		Content   interface{} `json:"content"`
		Timestamp string      `json:"timestamp"`
	} `json:"messages"`
}

type Session struct {
	client        *api.ApiClient
	reader        *bufio.Reader
	state         SessionState
	currentConfig *ChatConfig
	provider      brunch.Provider
}

func NewSession(client *api.ApiClient) *Session {
	return &Session{
		client: client,
		reader: bufio.NewReader(os.Stdin),
		state:  SSSelection,
	}
}

func (s *Session) Start() error {
	for {
		switch s.state {
		case SSSelection:
			if err := s.handleSelectionState(); err != nil {
				return err
			}
		case SSInteraction:
			if err := s.handleInteractionState(); err != nil {
				if err == errQuit {
					continue // Return to selection state
				}
				return err
			}
		}
	}
}

func (s *Session) handleSelectionState() error {
	for {
		fmt.Print("[-] > ")
		input, err := s.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if err := s.handleCommand(input); err != nil {
			if err == errQuit {
				return nil
			}
			fmt.Printf("Error: %v\n", err)
		}
	}
}

var errQuit = fmt.Errorf("quit")

func (s *Session) handleCommand(input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	switch strings.ToLower(cmd) {
	case "help":
		s.printHelp()
		return nil
	case "quit", "exit":
		return errQuit
	case "get":
		if len(args) != 1 {
			return fmt.Errorf("usage: get <key>")
		}
		return s.handleGet(args[0])
	case "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: set <key> <value>")
		}
		value := strings.Join(args[1:], " ")
		return s.handleSet(args[0], value)
	case "delete":
		if len(args) != 1 {
			return fmt.Errorf("usage: delete <key>")
		}
		return s.handleDelete(args[0])
	case "new":
		if len(args) != 1 {
			return fmt.Errorf("usage: new <name>")
		}
		return s.handleNewChat(args[0])
	case "load":
		if len(args) != 1 {
			return fmt.Errorf("usage: load <name>")
		}
		return s.handleLoadChat(args[0])
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (s *Session) printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  help              - Show this help message")
	fmt.Println("  get <key>         - Get value for key")
	fmt.Println("  set <key> <value> - Set value for key")
	fmt.Println("  delete <key>      - Delete key")
	fmt.Println("  new <name>        - Create new chat configuration")
	fmt.Println("  load <name>       - Load existing chat configuration")
	fmt.Println("  quit/exit         - Exit the program")
}

func (s *Session) handleGet(key string) error {
	resp, err := s.client.Query(api.BrunchOpRead, key, "")
	if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}
	fmt.Printf("%s = %s\n", key, resp.Result)
	return nil
}

func (s *Session) handleSet(key, value string) error {
	resp, err := s.client.Query(api.BrunchOpCreate, key, value)
	if err != nil {
		return fmt.Errorf("failed to set value: %w", err)
	}
	fmt.Printf("Set %s = %s\n", key, resp.Result)
	return nil
}

func (s *Session) handleDelete(key string) error {
	_, err := s.client.Query(api.BrunchOpDelete, key, "")
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	fmt.Printf("Deleted %s\n", key)
	return nil
}

func (s *Session) handleNewChat(name string) error {
	// Create default chat config
	config := ChatConfig{
		Name:        name,
		Model:       anthropic.DefaultModel,
		Temperature: 0.7,
		MaxTokens:   4096,
		Prompt:      "You are a helpful AI assistant.",
		Messages: []struct {
			Role      string      "json:\"role\""
			Content   interface{} "json:\"content\""
			Timestamp string      "json:\"timestamp\""
		}{},
	}

	// Convert to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Store in KV store with chat: prefix
	chatKey := "chat:" + name
	if _, err := s.client.Query(api.BrunchOpCreate, chatKey, string(configJSON)); err != nil {
		return fmt.Errorf("failed to create chat: %w", err)
	}

	// Create Anthropic client
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	client, err := anthropic.New(apiKey, config.Prompt, config.Temperature, config.MaxTokens)
	if err != nil {
		return fmt.Errorf("failed to create Anthropic client: %w", err)
	}

	s.provider = anthropic.NewAnthropicProvider(client)
	s.currentConfig = &config
	s.state = SSInteraction

	fmt.Printf("Created new chat configuration: %s\n", name)
	fmt.Println("Entering interaction mode. Use \\q to return to selection mode.")
	return errQuit
}

func (s *Session) handleLoadChat(name string) error {
	chatKey := "chat:" + name
	resp, err := s.client.Query(api.BrunchOpRead, chatKey, "")
	if err != nil {
		return fmt.Errorf("failed to load chat: %w", err)
	}

	var config ChatConfig
	if err := json.Unmarshal([]byte(resp.Result), &config); err != nil {
		return fmt.Errorf("failed to unmarshal chat config: %w", err)
	}

	// Create Anthropic client
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	client, err := anthropic.New(apiKey, config.Prompt, config.Temperature, config.MaxTokens)
	if err != nil {
		return fmt.Errorf("failed to create Anthropic client: %w", err)
	}

	s.provider = anthropic.NewAnthropicProvider(client)
	s.currentConfig = &config
	s.state = SSInteraction

	fmt.Printf("Loaded chat configuration: %s\n", name)
	fmt.Println("Entering interaction mode. Use \\q to return to selection mode.")
	return errQuit // Return errQuit to break out of selection state loop
}

func (s *Session) handleInteractionState() error {
	root := s.provider.NewNett()
	currentNode := root
	done := make(chan error)

	repl := brunch.NewRepl(brunch.PanelOpts{
		Provider: s.provider,
		PreHook: func(query *string) error {
			return nil
		},
		PostHook: func(response *string) error {
			fmt.Println("Response:", *response)
			return nil
		},
		InterruptHandler: func(node brunch.Node) {
			if n, ok := node.(*brunch.RootNode); ok {
				currentNode = *n
			}
		},
		CompletionHandler: func(node brunch.Node) {
			if n, ok := node.(*brunch.RootNode); ok {
				currentNode = *n
			}
		},
		Commands: brunch.CommandOpts{
			KeyOn: '\\',
			Handler: func(panel brunch.Panel, nodeHash, line string) error {
				parts := strings.Fields(line)
				if len(parts) == 0 {
					return nil
				}

				switch parts[0] {
				case "\\h":
					fmt.Printf("Current hash: [%s]\n", nodeHash)
				case "\\l":
					fmt.Println(panel.PrintHistory())
				case "\\t":
					fmt.Println(panel.PrintTree())
				case "\\r":
					fmt.Println("Routes:")
					routes := panel.GetRoutes()
					for k, v := range routes {
						fmt.Printf("%s: %s\n", k, v)
					}
					fmt.Println("Enter route key:")
					var route string
					fmt.Scanln(&route)
					if err := panel.TraverseToRoute(routes[route]); err != nil {
						fmt.Println("Failed to traverse to route:", err)
					}
				case "\\i":
					fmt.Println("Enter image path:")
					var imagePath string
					fmt.Scanln(&imagePath)
					if err := panel.QueueImages([]string{imagePath}); err != nil {
						fmt.Println("Failed to queue image:", err)
					}
				case "\\s":
					if err := s.saveCurrentState(&currentNode); err != nil {
						fmt.Printf("Failed to save state: %v\n", err)
					} else {
						fmt.Println("State saved successfully")
					}
				case "\\q":
					s.state = SSSelection
					done <- errQuit
					return nil
				}
				return nil
			},
		},
	})

	go func() {
		repl.Run()
		done <- nil
	}()

	return <-done
}

func (s *Session) saveCurrentState(node *brunch.RootNode) error {
	if s.currentConfig == nil {
		return fmt.Errorf("no chat configuration loaded")
	}

	// Convert conversation history to config format
	history := s.provider.GetHistory(node)
	messages := make([]struct {
		Role      string      `json:"role"`
		Content   interface{} `json:"content"`
		Timestamp string      `json:"timestamp"`
	}, len(history))

	for i, msg := range history {
		messages[i].Role = msg["role"]
		messages[i].Content = msg["content"]
		messages[i].Timestamp = time.Now().Format(time.RFC3339)
	}

	s.currentConfig.Messages = messages

	// Convert to JSON
	configJSON, err := json.Marshal(s.currentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Store in KV store
	chatKey := "chat:" + s.currentConfig.Name
	if _, err := s.client.Query(api.BrunchOpUpdate, chatKey, string(configJSON)); err != nil {
		return fmt.Errorf("failed to save chat: %w", err)
	}

	return nil
}
