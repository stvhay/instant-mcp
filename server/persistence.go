package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hays/instant-mcp/models"
)

// StateFile represents the persisted state format
type StateFile struct {
	Version  string                    `json:"version"`
	Commands map[string]models.Command `json:"commands"`
}

// LoadState loads the registry state from a JSON file.
// Returns empty state if file doesn't exist or is corrupted.
func LoadState(path string) (map[string]models.Command, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		log.Printf("No state file found at %s, starting fresh", path)
		return make(map[string]models.Command), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupted file - back it up and start fresh
		backupPath := path + ".bak"
		os.Rename(path, backupPath)
		log.Printf("State file corrupted, backed up to %s, starting fresh", backupPath)
		return make(map[string]models.Command), nil
	}

	if state.Commands == nil {
		state.Commands = make(map[string]models.Command)
	}

	log.Printf("Loaded %d commands from %s", len(state.Commands), path)
	return state.Commands, nil
}

// SaveState persists the registry state to a JSON file
func SaveState(path string, commands map[string]models.Command) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	state := StateFile{
		Version:  "1.0",
		Commands: commands,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}
