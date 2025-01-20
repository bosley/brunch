package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bosley/brunch"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	doInitDir := flag.String("init", "", "Initialize the config file")
	loadDir := flag.String("load", ".", "Load directory containing insu.yaml")
	flag.Parse()

	if *doInitDir != "" {
		if err := InitDirectory(*doInitDir); err != nil {
			fmt.Println("Failed to initialize directory:", err)
			os.Exit(1)
		}
		*loadDir = *doInitDir
	}

	config, err := LoadFromDir(*loadDir)
	if err != nil {
		fmt.Println("Failed to load config:", err)

		fmt.Println("please use the -init flag to initialize the config file (see --help for more info)")
		os.Exit(1)
	}

	client := clientFromSelectedProvider(config)

	brunch.NewRepl(brunch.PanelOpts{
		Provider:          client,
		PreHook:           preHook,
		PostHook:          postHook,
		InterruptHandler:  interruptHandler,
		CompletionHandler: completionHandler,
		Commands: brunch.CommandOpts{
			KeyOn:   brunch.DefaultCommandKey,
			Handler: handleCommand,
		},
	}).Run()
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
	case "\\h":
		fmt.Printf("Current hash: [%s]", nodeHash)
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
	}
	return nil
}
