package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bosley/brunch"
)

var loadDir *string
var restore *string

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
	}
	return nil
}
