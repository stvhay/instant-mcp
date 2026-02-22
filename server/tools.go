package server

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hays/instant-mcp/models"
)

// handleToolsList returns all available tools (built-in + dynamic)
func (s *Server) handleToolsList(msg *JSONRPCMessage) error {
	tools := s.builtinTools()

	// Add dynamic tools from registry
	for _, cmd := range s.registry.List() {
		tools = append(tools, commandToTool(cmd))
	}

	result := struct {
		Tools []Tool `json:"tools"`
	}{Tools: tools}

	return s.transport.WriteResponse(msg.ID, result)
}

// handleToolsCall dispatches a tool call to the appropriate handler
func (s *Server) handleToolsCall(msg *JSONRPCMessage) error {
	var params ToolsCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return fmt.Errorf("invalid tools/call params: %w", err)
	}

	log.Printf("Tool call: %s", params.Name)

	// Check built-in tools first
	if handler, ok := s.builtinHandlers()[params.Name]; ok {
		return handler(msg, params)
	}

	// Check dynamic commands
	cmd, err := s.registry.Get(params.Name)
	if err != nil {
		return s.transport.WriteError(msg.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name), nil)
	}

	// Execute the command
	output, execErr := Execute(cmd, params.Arguments)
	if execErr != nil {
		errMsg := execErr.Error()
		if output != "" {
			errMsg = output + "\n" + errMsg
		}
		return s.respondError(msg.ID, errMsg)
	}

	if output == "" {
		output = "(no output)"
	}
	return s.respondText(msg.ID, output)
}

// commandToTool converts a Command to an MCP Tool definition
func commandToTool(cmd models.Command) Tool {
	props := make(map[string]any)
	var required []string

	for argName, arg := range cmd.Args {
		prop := map[string]any{
			"type": arg.Type,
		}
		if arg.Description != "" {
			prop["description"] = arg.Description
		}
		props[argName] = prop

		if arg.Required {
			required = append(required, argName)
		}
	}

	return Tool{
		Name:        cmd.Name,
		Description: cmd.Description,
		InputSchema: InputSchema{
			Type:       "object",
			Properties: props,
			Required:   required,
		},
	}
}

// builtinHandlers returns the dispatch map for built-in tool handlers
func (s *Server) builtinHandlers() map[string]toolHandler {
	return map[string]toolHandler{
		"help":           s.handleHelp,
		"add_command":    s.handleAddCommand,
		"remove_command": s.handleRemoveCommand,
		"list_commands":  s.handleListCommands,
		"get_command":    s.handleGetCommand,
		"batch_exec":     s.handleBatchExec,
		"update_command": s.handleUpdateCommand,
		"import_config":  s.handleImportConfig,
		"export_config":  s.handleExportConfig,
	}
}

// builtinTools returns the schema definitions for all built-in tools
func (s *Server) builtinTools() []Tool {
	return []Tool{
		{
			Name:        "help",
			Description: "Get usage guide for instant-mcp. Call this first to learn how to register and use dynamic commands.",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "add_command",
			Description: "Register a new command as an MCP tool by wrapping an executable.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Unique command name (alphanumeric and underscores, must start with letter)",
					},
					"exec": map[string]any{
						"type":        "string",
						"description": "Path to executable (absolute, relative to cwd, or in $PATH)",
					},
					"args": map[string]any{
						"type":        "object",
						"description": "Argument specifications: {\"arg_name\": {\"type\": \"string|number|boolean\", \"description\": \"...\", \"required\": true}}",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "Help text shown to agents",
					},
					"async": map[string]any{
						"type":        "boolean",
						"description": "Run asynchronously (default: false)",
					},
					"timeout": map[string]any{
						"type":        "string",
						"description": "Timeout duration, e.g. '30s', '5m', '1h' (default: '120s')",
					},
				},
				Required: []string{"name", "exec"},
			},
		},
		{
			Name:        "remove_command",
			Description: "Unregister a command by name.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the command to remove",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "list_commands",
			Description: "List all registered commands with their descriptions.",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "get_command",
			Description: "Get full details of a registered command.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the command to inspect",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "batch_exec",
			Description: "Execute multiple command operations atomically. Supports add_command, remove_command, and update_command operations in a single call.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"commands": map[string]any{
						"type":        "array",
						"description": "Array of operations: [{\"operation\": \"add_command\"|\"remove_command\"|\"update_command\", \"params\": {...}}]",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"operation": map[string]any{
									"type": "string",
									"enum": []string{"add_command", "remove_command", "update_command"},
								},
								"params": map[string]any{
									"type": "object",
								},
							},
							"required": []string{"operation", "params"},
						},
					},
					"atomic": map[string]any{
						"type":        "boolean",
						"description": "If true (default), all operations succeed or all fail. If false, partial success is allowed.",
					},
				},
				Required: []string{"commands"},
			},
		},
		{
			Name:        "update_command",
			Description: "Update an existing registered command. Provide name of command to update plus any fields to change.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the command to update",
					},
					"exec": map[string]any{
						"type":        "string",
						"description": "New executable path",
					},
					"args": map[string]any{
						"type":        "object",
						"description": "New argument specifications (replaces existing args)",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "New help text",
					},
					"async": map[string]any{
						"type":        "boolean",
						"description": "New async setting",
					},
					"timeout": map[string]any{
						"type":        "string",
						"description": "New timeout duration",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "import_config",
			Description: "Bulk import commands from a YAML or JSON file. Existing commands with the same name are skipped unless overwrite is true.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to YAML or JSON file containing commands",
					},
					"overwrite": map[string]any{
						"type":        "boolean",
						"description": "If true, overwrite existing commands with same name (default: false)",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "export_config",
			Description: "Export all registered commands to a YAML file for version control or backup.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Output file path (default: .instant-mcp/commands.yaml)",
					},
				},
			},
		},
	}
}
