package main

import (
	"bufio"
	"crypto"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/bosley/brunch"
	"github.com/bosley/brunch/anthropic"
)

var loadDir *string
var chatEnabled bool
var core *brunch.Core
var logger *slog.Logger

const sessionId = "cli-session"

func main() {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
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
		slog.Info("installing core", "dir", *loadDir)
		if err := core.Install(); err != nil {
			fmt.Println("Failed to install core:", err)
			os.Exit(1)
		}
	} else {
		slog.Info("core already installed, loading providers", "dir", *loadDir)
		if err := core.LoadProviders(); err != nil {
			slog.Error("failed to load providers", "error", err)
			os.Exit(1)
		}
		if err := core.LoadContexts(); err != nil {
			slog.Error("failed to load contexts", "error", err)
			os.Exit(1)
		}
	}
	doRepl()
}

func doRepl() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(">")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		// Quick check for immediate exit
		statement := strings.TrimSpace(line)
		if isNonReplQuit(statement) {
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
			doChat(req.ChatRequest.LoadedInstance)
		}
	}
}

// Perform the actual chat with the person. This will eventually be diffused into a server
// that could be repld if I decide to make this a web app.
func doChat(chat brunch.Conversation) {

	banner()

	chatEnabled = true
	chat.ToggleChat(chatEnabled)

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Chat started. Press Ctrl+C to exit and view conversation tree.")
	fmt.Println("Enter your messages (press Enter twice to send):")

	for {
		var lines []string
		currentHash := chat.CurrentNode().Hash()[:8]
		fmt.Printf("\n[%s]>  ", currentHash)

		// Read until double Enter
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				slog.Error("error reading input", "error", err)
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
						slog.Error("command failed", "error", err)
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
			fmt.Println("chat is disabled, skipping. use \\x to toggle")
			continue
		}

		question := strings.Join(lines, "\n")
		response, err := chat.SubmitMessage(question)
		if err != nil {
			slog.Error("failed to submit message", "error", err)
			continue
		}

		fmt.Println("assistant> ", response)
	}
}

func handleCommand(conversation brunch.Conversation, line string) (bool, error) {
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
		fmt.Println("\t\\a: List artifacts [display artifacts from current node] or [write artifacts to disk if followed by a directory path]")
		fmt.Println("\t\\q: Quit [save and quit]")

		// Added for convenience, so we don't have to exit the current chat to add a new context to the core
		// When a context is added via a chat, it is automatically saved to disk and will be mandatory for the chat
		// to be restored in the future.
		fmt.Println("\t\\new-k: Attach new knowledge-context [attach a non-existing knowledge-context to the chat]")
		fmt.Println("\t\\attach-k: Attach existing knowledge-context [attach an existing knowledge-context to the chat]")
	case "\\l":
		fmt.Println(conversation.PrintHistory())
	case "\\t":
		fmt.Println(conversation.PrintTree())
	case "\\i":
		fmt.Println("Enter image path:")
		var imagePath string
		fmt.Scanln(&imagePath)
		if err := conversation.QueueImages([]string{imagePath}); err != nil {
			fmt.Println("Failed to queue image:", err)
			return true, err
		}
	case "\\s":
		saveSnapshot()
	case "\\p":
		if err := conversation.Parent(); err != nil {
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
		if err := conversation.Child(idx); err != nil {
			fmt.Println("failed to go to child", err)
			return true, err
		}
	case "\\r":
		if err := conversation.Root(); err != nil {
			fmt.Println("failed to go to root", err)
			return true, err
		}
	case "\\g":
		if len(parts) < 2 {
			fmt.Println("usage: \\g <node_hash>")
			return false, nil
		}
		if err := conversation.Goto(parts[1]); err != nil {
			fmt.Println("failed to go to node", err)
			return true, err
		}
	case "\\.":
		if conversation.HasParent() {
			fmt.Println("current node has parent; use \\p to access")
		}
		children := conversation.ListChildren()
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
		conversation.ToggleChat(chatEnabled)
		fmt.Printf("chat enabled: %t\n", chatEnabled)
	case "\\a":
		return handleArtifacting(conversation, parts)
	case "\\new-k":
		if len(parts) < 4 {
			fmt.Println("usage: \\new-k <name> <type> <value>")
			return false, nil
		}
		ctxName := parts[1]
		ctxType := parts[2]
		ctxValue := parts[3]

		if ctxType != string(brunch.ContextTypeDirectory) &&
			ctxType != string(brunch.ContextTypeDatabase) &&
			ctxType != string(brunch.ContextTypeWeb) {
			fmt.Println(
				"invalid context type",
				ctxType,
				"must be one of:",
				strings.Join([]string{
					string(brunch.ContextTypeDirectory),
					string(brunch.ContextTypeDatabase),
					string(brunch.ContextTypeWeb),
				}, ", "),
			)
			return false, nil
		}

		ctx := &brunch.ContextSettings{
			Name:  ctxName,
			Type:  brunch.ContextType(ctxType),
			Value: ctxValue,
		}
		if err := conversation.CreateContext(ctx); err != nil {
			fmt.Println("failed to attach context", err)
			return true, err
		}
		fmt.Println("attached context", ctxName, "to chat")

	case "\\attach-k":
		if len(parts) < 2 {
			fmt.Println("usage: \\attach-k <name>")
			return false, nil
		}
		ctxName := parts[1]
		if err := conversation.AttachContext(ctxName); err != nil {
			fmt.Println("failed to attach context", err)
			return true, err
		}
		fmt.Println("attached context", ctxName, "to chat")
	case "\\available-k":
		fmt.Println("Available Knowledge Contexts:\n")
		for _, ctx := range core.ListContexts() {
			fmt.Println("\t", ctx)
		}
	case "\\active-k":
		fmt.Println("Active Knowledge Contexts:\n")
		for _, ctx := range conversation.ListKnowledgeContexts() {
			fmt.Println("\t", ctx)
		}
	case "\\q":
		fmt.Println("saving back to loaded snapshot")
		if err := saveSnapshot(); err != nil {
			slog.Error("failed to save snapshot on quit", "error", err)
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

func banner() {
	fmt.Println(`

		        W E L C O M E

		Send a message to the assistant by
		typing the message and pressing "enter"
		twice.

		To see a list of commands type '\?'
		To quit, type '\q'
		`)
}

func isNonReplQuit(line string) bool {
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

func handleArtifacting(conversation brunch.Conversation, parts []string) (bool, error) {

	artifacts := conversation.Artifacts()
	if len(artifacts) == 0 {
		fmt.Println("No artifacts in current node")
		return false, nil
	}

	writeToDisk := (len(parts) == 2)

	if writeToDisk {
		// Ensure the target is a directory
		if fi, err := os.Stat(parts[1]); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(parts[1], 0755); err != nil {
					return false, nil
				}
			}
			if fi != nil && !fi.IsDir() {
				fmt.Println("Target is not a directory")
				return false, nil
			}
		}
	}

	if !writeToDisk {
		fmt.Println("Artifacts in current node:")
	}
	for i, artifact := range artifacts {
		switch artifact.Type() {
		case brunch.ArtifactTypeFile:
			if fa, ok := artifact.(*brunch.FileArtifact); ok {
				if writeToDisk {
					name := fmt.Sprintf("file_%s.artifact", fa.Id)
					if fa.Name != "" {
						name = fa.Name
					}
					if err := fa.Write(parts[1], name); err != nil {
						fmt.Println("failed to write artifact", fa.Id, "to disk at location", parts[1])
					}
				} else {
					// Just show the previews
					fileType := "unknown"
					if fa.FileType != nil {
						fileType = *fa.FileType
					}
					name := "<unnamed artifact>"
					if fa.Name != "" {
						name = fa.Name
					}
					preview := fa.Data
					if len(preview) > 50 {
						preview = preview[:50] + "..."
					}
					fmt.Printf("\t%d: File [%s] Name: %s\n\t   Preview: %s\n", i, fileType, name, preview)
				}
			}
		case brunch.ArtifactTypeNonFile:
			if nfa, ok := artifact.(*brunch.NonFileArtifact); ok {
				if writeToDisk {
					// get sha256 hash of the data
					hash := crypto.SHA256.New()
					hash.Write([]byte(artifact.(*brunch.NonFileArtifact).Data))
					sum := fmt.Sprintf("%x", hash.Sum(nil))
					name := fmt.Sprintf("%s.artifact", sum[:8]) // Use first 8 chars of hex-encoded hash
					if err := nfa.Write(parts[1], name); err != nil {
						fmt.Println("failed to write non-file artifact", name, "to disk at location", parts[1])
					}
				} else {
					preview := nfa.Data
					if len(preview) > 50 {
						preview = preview[:50] + "..."
					}
					fmt.Printf("\t%d: Text: %s\n", i, preview)
				}
			}
		}
	}
	return false, nil
}
