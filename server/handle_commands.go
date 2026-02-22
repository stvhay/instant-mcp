package server

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hays/instant-mcp/models"
)

func (s *Server) handleHelp(msg *JSONRPCMessage, _ ToolsCallParams) error {
	help := `# instant-mcp Usage Guide

instant-mcp lets you register executables as MCP tools at runtime.

## Quick Start

1. Add a command:
   add_command(name: "greet", exec: "./scripts/greet.sh", args: {"name": {"type": "string", "required": true}}, description: "Greet someone")

2. The command immediately appears as an MCP tool.

3. Call it: greet(name: "world")

## Tools

- add_command     - Register a new command
- remove_command  - Unregister a command
- update_command  - Modify an existing command
- list_commands   - Show all registered commands
- get_command     - Show command details
- batch_exec      - Multiple operations atomically
- import_config   - Bulk import from YAML/JSON file
- export_config   - Export commands to YAML for version control
- help            - This guide

## Batch Setup

Register multiple commands in one call:
  batch_exec(commands: [
    {"operation": "add_command", "params": {"name": "lint", "exec": "./scripts/lint.sh"}},
    {"operation": "add_command", "params": {"name": "test", "exec": "./scripts/test.sh"}}
  ], atomic: true)

## Argument Types

- "string"  - Text input
- "number"  - Numeric input
- "boolean" - true/false

## Timeouts

Set per-command: "30s", "5m", "1h". Default: 120s.

## Version Control

Export: export_config(path: ".instant-mcp/commands.yaml")
Import: import_config(path: ".instant-mcp/commands.yaml")

## Security

Commands run with the server's permissions. Only register trusted executables.`

	return s.respondText(msg.ID, help)
}

func (s *Server) handleAddCommand(msg *JSONRPCMessage, params ToolsCallParams) error {
	cmd, err := parseCommand(params.Arguments)
	if err != nil {
		return s.respondError(msg.ID, err.Error())
	}

	if err := s.registry.Add(cmd); err != nil {
		return s.respondError(msg.ID, err.Error())
	}

	s.persist()
	log.Printf("Added command: %s -> %s", cmd.Name, cmd.Exec)
	return s.respondText(msg.ID, fmt.Sprintf("Command %q registered successfully. It is now available as an MCP tool.", cmd.Name))
}

func (s *Server) handleRemoveCommand(msg *JSONRPCMessage, params ToolsCallParams) error {
	name, _ := params.Arguments["name"].(string)
	if name == "" {
		return s.respondError(msg.ID, "name is required")
	}

	if err := s.registry.Remove(name); err != nil {
		return s.respondError(msg.ID, err.Error())
	}

	s.persist()
	log.Printf("Removed command: %s", name)
	return s.respondText(msg.ID, fmt.Sprintf("Command %q removed.", name))
}

func (s *Server) handleListCommands(msg *JSONRPCMessage, _ ToolsCallParams) error {
	cmds := s.registry.List()

	if len(cmds) == 0 {
		return s.respondText(msg.ID, "No commands registered. Use add_command to register one.")
	}

	data, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return s.respondError(msg.ID, fmt.Sprintf("failed to marshal commands: %v", err))
	}

	return s.respondText(msg.ID, string(data))
}

func (s *Server) handleGetCommand(msg *JSONRPCMessage, params ToolsCallParams) error {
	name, _ := params.Arguments["name"].(string)
	if name == "" {
		return s.respondError(msg.ID, "name is required")
	}

	cmd, err := s.registry.Get(name)
	if err != nil {
		return s.respondError(msg.ID, err.Error())
	}

	data, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return s.respondError(msg.ID, fmt.Sprintf("failed to marshal command: %v", err))
	}

	return s.respondText(msg.ID, string(data))
}

func (s *Server) handleUpdateCommand(msg *JSONRPCMessage, params ToolsCallParams) error {
	name, _ := params.Arguments["name"].(string)
	if name == "" {
		return s.respondError(msg.ID, "name is required")
	}

	// Get existing command as base
	existing, err := s.registry.Get(name)
	if err != nil {
		return s.respondError(msg.ID, err.Error())
	}

	// Apply updates
	if exec, ok := params.Arguments["exec"].(string); ok {
		existing.Exec = exec
	}
	if desc, ok := params.Arguments["description"].(string); ok {
		existing.Description = desc
	}
	if async, ok := params.Arguments["async"].(bool); ok {
		existing.Async = async
	}
	if timeout, ok := params.Arguments["timeout"].(string); ok {
		existing.Timeout = timeout
	}
	if argsRaw, ok := params.Arguments["args"].(map[string]any); ok {
		existing.Args = make(map[string]models.Arg)
		for argName, argVal := range argsRaw {
			argMap, ok := argVal.(map[string]any)
			if !ok {
				return s.respondError(msg.ID, fmt.Sprintf("arg %q must be an object", argName))
			}
			arg := models.Arg{}
			arg.Type, _ = argMap["type"].(string)
			arg.Description, _ = argMap["description"].(string)
			if req, ok := argMap["required"].(bool); ok {
				arg.Required = req
			}
			existing.Args[argName] = arg
		}
	}

	if err := s.registry.Update(name, existing); err != nil {
		return s.respondError(msg.ID, err.Error())
	}

	s.persist()
	log.Printf("Updated command: %s", name)
	return s.respondText(msg.ID, fmt.Sprintf("Command %q updated.", name))
}

// parseCommand extracts a Command from tool call arguments
func parseCommand(args map[string]any) (models.Command, error) {
	cmd := models.Command{
		Timeout: "120s",
	}

	name, _ := args["name"].(string)
	cmd.Name = name

	exec, _ := args["exec"].(string)
	cmd.Exec = exec

	if desc, ok := args["description"].(string); ok {
		cmd.Description = desc
	}

	if async, ok := args["async"].(bool); ok {
		cmd.Async = async
	}

	if timeout, ok := args["timeout"].(string); ok {
		cmd.Timeout = timeout
	}

	if argsRaw, ok := args["args"].(map[string]any); ok {
		cmd.Args = make(map[string]models.Arg)
		for argName, argVal := range argsRaw {
			argMap, ok := argVal.(map[string]any)
			if !ok {
				return cmd, fmt.Errorf("arg %q must be an object with type, description, and required fields", argName)
			}
			arg := models.Arg{}
			arg.Type, _ = argMap["type"].(string)
			arg.Description, _ = argMap["description"].(string)
			if req, ok := argMap["required"].(bool); ok {
				arg.Required = req
			}
			cmd.Args[argName] = arg
		}
	}

	return cmd, nil
}
