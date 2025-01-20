package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bosley/brunch/api"
)

type SessionState string

const (
	SSSelection   SessionState = "selection"   // User can CRUD their KV datastore or LOAD a convo from it
	SSInteraction SessionState = "interaction" // User is in an interactive conversation
)

type Session struct {
	client *api.ApiClient
	reader *bufio.Reader
	state  SessionState
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

func (s *Session) handleInteractionState() error {

	return nil
}
