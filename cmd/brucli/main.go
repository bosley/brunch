package main

import (
	"bufio"
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

var baseProviders = map[string]brunch.Provider{
	"anthropic": anthropic.InitialAnthropicProvider(),
}

const sessionId = "cli-chat"

const (
	DefaultCommandKey uint8 = '\\'
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	loadDir = flag.String("load", ".", "Load directory containing insu.yaml")
	flag.Parse()

	core = brunch.NewCore(brunch.CoreOpts{
		InstallDirectory: *loadDir,
		BaseProviders:    baseProviders,
	})

	if !core.IsInstalled() {
		if err := core.Install(); err != nil {
			fmt.Println("Failed to install core:", err)
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

		statement := strings.TrimSpace(line)
		if statement == "\\quit" || statement == "\\exit" {
			return
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

		if req.ChatRequest != nil {
			doChat(req.ChatRequest.LoadedInstance)
		}
	}
}

func doChat(chat *brunch.ChatInstance) {

	welcome()
	chatEnabled = true
	chat.ToggleChat(chatEnabled)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool)

	// Start chat loop in goroutine
	go func() {
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
					fmt.Printf("Error reading input: %v\n", err)
					done <- true
					return
				}

				line = strings.TrimSpace(line)
				if line == "" && len(lines) > 0 {
					break
				}
				if line != "" {
					if strings.HasPrefix(line, string(DefaultCommandKey)) {
						if err := handleCommand(chat, line); err != nil {
							fmt.Println("Command failed:", err)
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
	}()

	select {
	case <-sigChan:
		if err := saveSnapshot(); err != nil {
			fmt.Println("failed to save snapshot on interrupt:", err)
		}
	case <-done:
		if err := saveSnapshot(); err != nil {
			fmt.Println("failed to save snapshot on completion:", err)
		}
	}
}

func handleCommand(panel brunch.Panel, line string) error {
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
			return err
		}
	case "\\s":
		saveSnapshot()
	case "\\p":
		if err := panel.Parent(); err != nil {
			fmt.Println("failed to go to parent", err)
			return err
		}
	case "\\c":
		if len(parts) < 2 {
			fmt.Println("usage: \\c <index>")
			return nil
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil {
			fmt.Println("failed to parse index", err)
			return err
		}
		if err := panel.Child(idx); err != nil {
			fmt.Println("failed to go to child", err)
			return err
		}
	case "\\r":
		if err := panel.Root(); err != nil {
			fmt.Println("failed to go to root", err)
			return err
		}
	case "\\g":
		if len(parts) < 2 {
			fmt.Println("usage: \\g <node_hash>")
			return nil
		}
		if err := panel.Goto(parts[1]); err != nil {
			fmt.Println("failed to go to node", err)
			return err
		}
	case "\\.":
		if panel.HasParent() {
			fmt.Println("current node has parent; use \\p to access")
		}
		children := panel.ListChildren()
		if len(children) == 0 {
			fmt.Println("current node has no children")
			return nil
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
			return nil
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
			os.Exit(1)
		}
		fmt.Println("quit")
		os.Exit(0)
	}
	return nil
}

func saveSnapshot() error {
	return core.SaveActiveChat(sessionId)
}

func welcome() {
	fmt.Println(`

	        W E L C O M E

	To see a list of commands type '\?'

	`)
}
