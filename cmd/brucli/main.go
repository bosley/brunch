package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/bosley/brunch"
	"github.com/bosley/brunch/anthropic"
)

var loadDir *string
var chatEnabled bool
var core *brunch.Core

var sigChan chan os.Signal
var done chan bool

const sessionId = "cli-session"

func main() {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	done = make(chan bool)

	// Handle signals in a separate goroutine
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
		done <- true
	}()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	loadDir = flag.String("load", "/tmp/brunch", "Load directory containing insu.yaml")
	flag.Parse()

	core = brunch.NewCore(brunch.CoreOpts{
		InstallDirectory: *loadDir,

		// These are not saved to disk - only derivatives are saved
		BaseProviders: map[string]brunch.Provider{
			"anthropic": anthropic.InitialAnthropicProvider(),
		},
	})

	if !core.IsInstalled() {

		fmt.Printf("first time installation for core in directory [%s]...\n", *loadDir)
		if err := core.Install(); err != nil {
			fmt.Println("Failed to install core:", err)
			os.Exit(1)
		}
	} else {

		fmt.Printf("loading providers from install directory [%s]...\n", *loadDir)
		if err := core.LoadProviders(); err != nil {
			fmt.Println("Failed to load providers:", err)
			os.Exit(1)
		}
	}

	fmt.Println("brunch cli started")
	for alive(ctx) {
		doRepl(ctx)
	}

	fmt.Println("exiting")
}

func doRepl(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)

	for alive(ctx) {
		fmt.Print(">")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		// Quick check for immediate exit
		statement := strings.TrimSpace(line)
		if isQuit(statement) {
			os.Exit(0)
		}

		// Check for "brunch statement"
		if !strings.HasPrefix(statement, "\\") {
			fmt.Println("invalid branch statement")
			continue
		}

		stmt := brunch.NewStatement(statement)
		if err := stmt.Prepare(); err != nil {
			fmt.Printf("Error preparing statement: %v\n", err)
			continue
		}

		req := core.ExecuteStatement(sessionId, stmt)
		if req.Error != nil {
			fmt.Printf("Error: %v\n", req.Error)
			continue
		}

		// If the statement yields anything other than an error, it's a chat request
		// as all other commands are handled by the core
		if req.ChatRequest != nil {
			doChat(ctx, req.ChatRequest.LoadedInstance)
			return
		}
	}
}

// Perform the actual chat with the person. This will eventually be diffused into a server
// that could be repld if I decide to make this a web app.
func doChat(ctx context.Context, chat *brunch.ChatInstance) {
	welcome()
	chatEnabled = false
	chat.ToggleChat(chatEnabled)

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Chat started. Press Ctrl+C to exit and view conversation tree.")
	fmt.Println("Enter your messages (press Enter twice to send):")

	for alive(ctx) {
		var lines []string
		currentHash := chat.CurrentNode().Hash()[:8]
		fmt.Printf("\n[%s]>  ", currentHash)

		// Read until double Enter
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				done <- true
				return
			}

			line = strings.TrimSpace(line)
			if line == "" && len(lines) > 0 {
				break
			}
			if line != "" {
				if strings.HasPrefix(line, "\\") {
					doQuit, err := handleCommand(chat, line)
					if err != nil {
						fmt.Println("Command failed:", err)
					}

					// Soft quit to exit the chat and go back to primary repl
					if doQuit {
						return
					}
					currentHash = chat.CurrentNode().Hash()[:8]
					fmt.Printf("\n[%s]>  ", currentHash)
				} else {
					lines = append(lines, line)
				}
			}
		}

		if !chatEnabled {
			fmt.Println("chat is disabled, skipping")
			continue
		}

		question := strings.Join(lines, "\n")
		response, err := chat.SubmitMessage(question)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Println("assistant> ", response)
	}
}

func handleCommand(panel brunch.Panel, line string) (bool, error) {
	parts := strings.Split(line, " ")
	switch parts[0] {
	case "\\?":
		fmt.Println("Commands:")
		fmt.Println("\t\\l: List chat history [current branch of chat]")
		fmt.Println("\t\\t: List chat tree [all branches]")
		fmt.Println("\t\\i: Queue image [import image file into chat for inquiry]")
		fmt.Println("\t\\s: Save snapshot [save a snapshot of the current tree to disk]")
		fmt.Println("\t\\p: Go to parent [traverse up the tree]")
		fmt.Println("\t\\c: Go to child [traverse down the tree to the nth child]")
		fmt.Println("\t\\r: Go to root [traverse to the root of the tree]")
		fmt.Println("\t\\g: Go to node [traverse to a specific node by hash]")
		fmt.Println("\t\\.: List children [list all children of the current node]")
		fmt.Println("\t\\x: Toggle chat [toggle chat mode on/off - chat on by default press enter twice to send with no command leading]")
		fmt.Println("\t\\a: List artifacts [display artifacts from current node]")
		fmt.Println("\t\\q: Quit [save and quit]")
	case "\\l":
		fmt.Println(panel.PrintHistory())
	case "\\t":
		fmt.Println(panel.PrintTree())
	case "\\i":
		fmt.Println("Enter image path:")
		var imagePath string
		fmt.Scanln(&imagePath)
		if err := panel.QueueImages([]string{imagePath}); err != nil {
			fmt.Println("Failed to queue image:", err)
			return true, err
		}
	case "\\s":
		saveSnapshot()
	case "\\p":
		if err := panel.Parent(); err != nil {
			fmt.Println("failed to go to parent", err)
			return true, err
		}
	case "\\c":
		if len(parts) < 2 {
			fmt.Println("usage: \\c <index>")
			return false, nil
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil {
			fmt.Println("failed to parse index", err)
			return true, err
		}
		if err := panel.Child(idx); err != nil {
			fmt.Println("failed to go to child", err)
			return true, err
		}
	case "\\r":
		if err := panel.Root(); err != nil {
			fmt.Println("failed to go to root", err)
			return true, err
		}
	case "\\g":
		if len(parts) < 2 {
			fmt.Println("usage: \\g <node_hash>")
			return false, nil
		}
		if err := panel.Goto(parts[1]); err != nil {
			fmt.Println("failed to go to node", err)
			return true, err
		}
	case "\\.":
		if panel.HasParent() {
			fmt.Println("current node has parent; use \\p to access")
		}
		children := panel.ListChildren()
		if len(children) == 0 {
			fmt.Println("current node has no children")
			return false, nil
		}
		fmt.Println("current node has children\n\tidx:\thash")
		for idx, child := range children {
			fmt.Printf("\t%d:\t%s\n", idx, child)
		}
		fmt.Println("\nuse \\c <idx> to go to child")
	case "\\x":
		chatEnabled = !chatEnabled
		panel.ToggleChat(chatEnabled)
		fmt.Printf("chat enabled: %t\n", chatEnabled)
	case "\\a":
		artifacts := panel.Artifacts()
		if len(artifacts) == 0 {
			fmt.Println("No artifacts in current node")
			return false, nil
		}
		fmt.Println("Artifacts in current node:")
		for i, artifact := range artifacts {
			switch artifact.Type() {
			case brunch.ArtifactTypeFile:
				if fa, ok := artifact.(*brunch.FileArtifact); ok {
					fileType := "unknown"
					if fa.FileType != nil {
						fileType = *fa.FileType
					}
					name := "(no name given)"
					if fa.Name != "" {
						name = fa.Name
					}
					preview := fa.Data
					if len(preview) > 50 {
						preview = preview[:50] + "..."
					}
					fmt.Printf("\t%d: File [%s] Name: %s\n\t   Preview: %s\n", i, fileType, name, preview)
				}
			case brunch.ArtifactTypeNonFile:
				if nfa, ok := artifact.(*brunch.NonFileArtifact); ok {
					preview := nfa.Data
					if len(preview) > 50 {
						preview = preview[:50] + "..."
					}
					fmt.Printf("\t%d: Text: %s\n", i, preview)
				}
			}
		}
	case "\\q":
		fmt.Println("saving back to loaded snapshot")
		if err := saveSnapshot(); err != nil {
			fmt.Println("failed to save snapshot", err)
		}
		return true, nil
	}
	return false, nil
}

// I made it this way to indicate that we saving due to the app
// call, and to because I had other save logic that I removed
// and uncle bob says short functions are lit
func saveSnapshot() error {
	return core.SaveActiveChat(sessionId)
}

func welcome() {
	fmt.Println(`

	        W E L C O M E

	To see a list of commands type '\?'

	`)
}

func isQuit(line string) bool {
	switch line {
	case "\\q":
		return true
	case "quit":
		return true
	case "exit":
		return true
	}
	return false
}

func alive(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		// We let this fail silently if it does as its a catchall for "just in case we need to save" scenario, but there might not be a chat session
		saveSnapshot()
		return false
	case <-done:
		// We let this fail silently if it does as its a catchall for "just in case we need to save" scenario, but there might not be a chat session
		saveSnapshot()
		return false
	default:
		return true
	}
}
