package server

import (
	"fmt"
	"maps"
	"regexp"
	"sync"

	"github.com/hays/instant-mcp/models"
)

var validName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// Registry stores registered commands in memory
type Registry struct {
	mu       sync.RWMutex
	commands map[string]models.Command
}

// NewRegistry creates an empty command registry
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]models.Command),
	}
}

// Add registers a new command. Returns error if name is taken or invalid.
func (r *Registry) Add(cmd models.Command) error {
	if err := validateCommand(cmd); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commands[cmd.Name]; exists {
		return fmt.Errorf("command %q already exists, use update to modify it", cmd.Name)
	}

	r.commands[cmd.Name] = cmd
	return nil
}

// Remove unregisters a command by name
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commands[name]; !exists {
		return fmt.Errorf("command %q not found", name)
	}

	delete(r.commands, name)
	return nil
}

// Get returns a command by name
func (r *Registry) Get(name string) (models.Command, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmd, exists := r.commands[name]
	if !exists {
		return models.Command{}, fmt.Errorf("command %q not found", name)
	}

	return cmd, nil
}

// List returns all registered commands
func (r *Registry) List() []models.Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmds := make([]models.Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// Update replaces an existing command
func (r *Registry) Update(name string, cmd models.Command) error {
	if err := validateCommand(cmd); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commands[name]; !exists {
		return fmt.Errorf("command %q not found", name)
	}

	// If name changed, remove old entry
	if name != cmd.Name {
		delete(r.commands, name)
	}

	r.commands[cmd.Name] = cmd
	return nil
}

// Snapshot returns a copy of all commands (for persistence)
func (r *Registry) Snapshot() map[string]models.Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := make(map[string]models.Command, len(r.commands))
	maps.Copy(snap, r.commands)
	return snap
}

// Load replaces the entire registry from a map (for loading from persistence)
func (r *Registry) Load(commands map[string]models.Command) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commands = make(map[string]models.Command, len(commands))
	maps.Copy(r.commands, commands)
}

// Len returns the number of registered commands
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.commands)
}

func validateCommand(cmd models.Command) error {
	if cmd.Name == "" {
		return fmt.Errorf("command name is required")
	}
	if !validName.MatchString(cmd.Name) {
		return fmt.Errorf("command name %q is invalid: must start with a letter, contain only letters, numbers, and underscores", cmd.Name)
	}
	if cmd.Exec == "" {
		return fmt.Errorf("exec is required for command %q", cmd.Name)
	}

	// Validate arg types
	validTypes := map[string]bool{"string": true, "number": true, "boolean": true}
	for argName, arg := range cmd.Args {
		if arg.Type == "" {
			return fmt.Errorf("arg %q in command %q must have a type", argName, cmd.Name)
		}
		if !validTypes[arg.Type] {
			return fmt.Errorf("arg %q in command %q has invalid type %q (must be string, number, or boolean)", argName, cmd.Name, arg.Type)
		}
	}

	// Validate timeout format if provided
	if cmd.Timeout != "" {
		if err := validateTimeout(cmd.Timeout); err != nil {
			return fmt.Errorf("command %q: %w", cmd.Name, err)
		}
	}

	return nil
}

func validateTimeout(timeout string) error {
	matched, _ := regexp.MatchString(`^\d+[smh]$`, timeout)
	if !matched {
		return fmt.Errorf("invalid timeout %q (use format like '30s', '5m', '1h')", timeout)
	}
	return nil
}
