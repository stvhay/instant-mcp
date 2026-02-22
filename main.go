package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/hays/instant-mcp/server"
)

const (
	version = "0.1.0"
	name    = "instant-mcp"
)

func main() {
	stateFile := flag.String("state-file", "", "Path to state file (default: ~/.instant-mcp/state.json)")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", name)
		fmt.Fprintf(os.Stderr, "A dynamic MCP server that lets agents register custom commands at runtime.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  INSTANT_MCP_STATE    Path to state file (overridden by --state-file)\n")
	}

	flag.Parse()

	// Logging to stderr (stdout is MCP protocol)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if *showVersion {
		fmt.Fprintf(os.Stderr, "%s v%s\n", name, version)
		os.Exit(0)
	}

	statePath := getStateFilePath(*stateFile)
	log.Printf("State file: %s", statePath)

	stateDir := filepath.Dir(statePath)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}

	srv := server.NewServer(name, version, statePath)
	if err := srv.LoadState(); err != nil {
		log.Printf("Warning: failed to load state: %v", err)
	}
	err := srv.Run()
	if errors.Is(err, io.EOF) {
		log.Printf("Client disconnected")
		return
	}
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func getStateFilePath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := os.Getenv("INSTANT_MCP_STATE"); envValue != "" {
		return envValue
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}
	return filepath.Join(home, ".instant-mcp", "state.json")
}
