package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bosley/brunch"
)

var loadDir *string
var restore *string
var chatEnabled bool

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	loadDir = flag.String("load", ".", "Load directory containing insu.yaml")
	restore = flag.String("snapshot", "", "Restore from snapshot")
	flag.Parse()

	var err error
	config, err := LoadFromDir(*loadDir)
	if err != nil {
		if err := InitDirectory(*loadDir); err != nil {
			fmt.Println("Failed to initialize directory:", err)
			os.Exit(1)
		}
		config, err = LoadFromDir(*loadDir)
		if err != nil {
			fmt.Println("Failed to load config:", err)
			os.Exit(1)
		}
	}

	client := clientFromSelectedProvider(config)

	brunchOpts := brunch.ReplOpts{
		Provider:          client,
		PreHook:           preHook,
		PostHook:          postHook,
		InterruptHandler:  interruptHandler,
		CompletionHandler: completionHandler,
		Commands: brunch.CommandOpts{
			KeyOn:   brunch.DefaultCommandKey,
			Handler: handleCommand,
		},
	}

	var repl *brunch.Repl
	if *restore != "" {
		snap, err := brunch.LoadSnapshot(*restore)
		if err != nil {
			fmt.Println("failed to load snapshot", err)
			os.Exit(1)
		}
		repl, err = brunch.NewReplFromSnapshot(brunchOpts, snap)
		if err != nil {
			fmt.Println("failed to restore snapshot", err)
			os.Exit(1)
		}
		fmt.Println("loaded snapshot")
	} else {
		repl = brunch.NewRepl(brunchOpts)
		fmt.Println("new chat")
	}

	chatEnabled = true
	repl.Run()
}

func preHook(query *string) error {
	fmt.Printf("PreHook: %s\n", *query)
	return nil
}

func postHook(response *string) error {
	fmt.Printf("PostHook: %s\n", *response)
	return nil
}

func interruptHandler(node brunch.Node) {
	fmt.Println("InterruptHandler", brunch.PrintTree(node))
}

func completionHandler(node brunch.Node) {
	fmt.Println("CompletionHandler", brunch.PrintTree(node))

}

func handleCommand(panel brunch.Panel, nodeHash, line string) error {
	fmt.Printf("handleCommand: %s\n", line)

	parts := strings.Split(line, " ")
	switch parts[0] {
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
		saveSnapshot(panel)
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
	case "\\q":
		if *restore != "" {
			fmt.Println("saving back to loaded snapshot")
			if err := saveSnapshot(panel); err != nil {
				fmt.Println("failed to save snapshot", err)
				os.Exit(1)
			}
		}
		fmt.Println("quit")
		os.Exit(0)
	}
	return nil
}

func saveSnapshot(panel brunch.Panel) error {
	snapshot, e := panel.Snapshot()
	if e != nil {
		fmt.Println("failed to take snapshot", e)
		return e
	}
	// Create a snapshot file with timestamp
	filename := fmt.Sprintf("snapshot-%d.json", time.Now().UnixMilli())
	if err := snapshot.Save(filename); err != nil {
		fmt.Println("failed to save snapshot", err)
		return err
	}
	return nil
}
