package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/hays/instant-mcp/models"
	"gopkg.in/yaml.v3"
)

// Server implements the MCP server
type Server struct {
	transport *Transport
	registry  *Registry
	name      string
	version   string
	statePath string
}

// NewServer creates a new MCP server
func NewServer(name, version, statePath string) *Server {
	return &Server{
		transport: NewTransport(),
		registry:  NewRegistry(),
		name:      name,
		version:   version,
		statePath: statePath,
	}
}

// LoadState loads persisted commands into the registry
func (s *Server) LoadState() error {
	commands, err := LoadState(s.statePath)
	if err != nil {
		return err
	}
	s.registry.Load(commands)
	return nil
}

// persist saves registry state to disk
func (s *Server) persist() {
	if err := SaveState(s.statePath, s.registry.Snapshot()); err != nil {
		log.Printf("Warning: failed to persist state: %v", err)
	}
}

// Run starts the server and processes messages
func (s *Server) Run() error {
	log.Printf("Starting %s v%s", s.name, s.version)

	for {
		msg, err := s.transport.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			return err
		}

		if err := s.handleMessage(msg); err != nil {
			log.Printf("Error handling message: %v", err)
			s.transport.WriteError(msg.ID, -32603, err.Error(), nil)
		}
	}
}

func (s *Server) handleMessage(msg *JSONRPCMessage) error {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "notifications/initialized":
		// Client acknowledgment, no response needed
		return nil
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(msg)
	default:
		if msg.ID != nil {
			return s.transport.WriteError(msg.ID, -32601, fmt.Sprintf("Method not found: %s", msg.Method), nil)
		}
		// Notifications without ID don't get responses
		return nil
	}
}

// --- Initialize ---

type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools map[string]any `json:"tools,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleInitialize(msg *JSONRPCMessage) error {
	var params InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return fmt.Errorf("invalid initialize params: %w", err)
	}

	log.Printf("Client: %s v%s", params.ClientInfo.Name, params.ClientInfo.Version)

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]any{},
		},
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}

	return s.transport.WriteResponse(msg.ID, result)
}

// --- Tool Types ---

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

type ToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type ToolsCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// --- tools/list ---

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

// --- tools/call ---

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

// --- Built-in Tools ---

type toolHandler func(msg *JSONRPCMessage, params ToolsCallParams) error

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

// --- Built-in Tool Handlers ---

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

// --- batch_exec ---

type batchOperation struct {
	Operation string         `json:"operation"`
	Params    map[string]any `json:"params"`
}

type batchResult struct {
	Index     int    `json:"index"`
	Operation string `json:"operation"`
	Name      string `json:"name,omitempty"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleBatchExec(msg *JSONRPCMessage, params ToolsCallParams) error {
	// Parse commands array
	cmdsRaw, ok := params.Arguments["commands"].([]any)
	if !ok || len(cmdsRaw) == 0 {
		return s.respondError(msg.ID, "commands must be a non-empty array")
	}

	// Default atomic=true
	atomic := true
	if a, ok := params.Arguments["atomic"].(bool); ok {
		atomic = a
	}

	// Parse operations
	ops := make([]batchOperation, 0, len(cmdsRaw))
	for i, raw := range cmdsRaw {
		opMap, ok := raw.(map[string]any)
		if !ok {
			return s.respondError(msg.ID, fmt.Sprintf("commands[%d] must be an object", i))
		}
		op := batchOperation{}
		op.Operation, _ = opMap["operation"].(string)
		if p, ok := opMap["params"].(map[string]any); ok {
			op.Params = p
		}
		if op.Operation == "" {
			return s.respondError(msg.ID, fmt.Sprintf("commands[%d] missing operation", i))
		}
		ops = append(ops, op)
	}

	if atomic {
		return s.batchAtomic(msg, ops)
	}
	return s.batchPartial(msg, ops)
}

func (s *Server) batchAtomic(msg *JSONRPCMessage, ops []batchOperation) error {
	// Take a snapshot for rollback
	snapshot := s.registry.Snapshot()

	results := make([]batchResult, 0, len(ops))
	for i, op := range ops {
		result := batchResult{Index: i, Operation: op.Operation}
		if name, _ := op.Params["name"].(string); name != "" {
			result.Name = name
		}

		if err := s.execBatchOp(op); err != nil {
			result.Error = err.Error()
			// Rollback
			s.registry.Load(snapshot)
			result.Success = false
			results = append(results, result)

			response := map[string]any{
				"success":     false,
				"rolled_back": true,
				"failed_at":   i,
				"error":       err.Error(),
				"results":     results,
			}
			data, _ := json.MarshalIndent(response, "", "  ")
			return s.respondError(msg.ID, string(data))
		}

		result.Success = true
		results = append(results, result)
	}

	s.persist()

	response := map[string]any{
		"success": true,
		"summary": fmt.Sprintf("%d/%d operations succeeded", len(results), len(results)),
		"results": results,
	}
	data, _ := json.MarshalIndent(response, "", "  ")
	return s.respondText(msg.ID, string(data))
}

func (s *Server) batchPartial(msg *JSONRPCMessage, ops []batchOperation) error {
	results := make([]batchResult, 0, len(ops))
	succeeded := 0

	for i, op := range ops {
		result := batchResult{Index: i, Operation: op.Operation}
		if name, _ := op.Params["name"].(string); name != "" {
			result.Name = name
		}

		if err := s.execBatchOp(op); err != nil {
			result.Error = err.Error()
			result.Success = false
		} else {
			result.Success = true
			succeeded++
		}
		results = append(results, result)
	}

	if succeeded > 0 {
		s.persist()
	}

	response := map[string]any{
		"success": succeeded == len(results),
		"summary": fmt.Sprintf("%d/%d operations succeeded", succeeded, len(results)),
		"results": results,
	}
	data, _ := json.MarshalIndent(response, "", "  ")

	if succeeded == len(results) {
		return s.respondText(msg.ID, string(data))
	}
	return s.respondError(msg.ID, string(data))
}

func (s *Server) execBatchOp(op batchOperation) error {
	switch op.Operation {
	case "add_command":
		cmd, err := parseCommand(op.Params)
		if err != nil {
			return err
		}
		return s.registry.Add(cmd)
	case "remove_command":
		name, _ := op.Params["name"].(string)
		if name == "" {
			return fmt.Errorf("name is required")
		}
		return s.registry.Remove(name)
	case "update_command":
		name, _ := op.Params["name"].(string)
		if name == "" {
			return fmt.Errorf("name is required")
		}
		cmd, err := parseCommand(op.Params)
		if err != nil {
			return err
		}
		return s.registry.Update(name, cmd)
	default:
		return fmt.Errorf("unknown operation: %s", op.Operation)
	}
}

// --- update_command ---

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

// --- import/export ---

// importFile represents the YAML/JSON format for import/export
type importFile struct {
	Commands map[string]models.Command `json:"commands" yaml:"commands"`
}

func (s *Server) handleImportConfig(msg *JSONRPCMessage, params ToolsCallParams) error {
	path, _ := params.Arguments["path"].(string)
	if path == "" {
		return s.respondError(msg.ID, "path is required")
	}

	overwrite := false
	if ow, ok := params.Arguments["overwrite"].(bool); ok {
		overwrite = ow
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s.respondError(msg.ID, fmt.Sprintf("failed to read file: %v", err))
	}

	var file importFile

	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, &file); err != nil {
		if err := json.Unmarshal(data, &file); err != nil {
			return s.respondError(msg.ID, "failed to parse file as YAML or JSON")
		}
	}

	if len(file.Commands) == 0 {
		return s.respondError(msg.ID, "no commands found in file")
	}

	imported, skipped := 0, 0
	var errors []string

	for _, cmd := range file.Commands {
		existing, _ := s.registry.Get(cmd.Name)
		if existing.Name != "" && !overwrite {
			skipped++
			continue
		}

		var opErr error
		if existing.Name != "" {
			opErr = s.registry.Update(cmd.Name, cmd)
		} else {
			opErr = s.registry.Add(cmd)
		}

		if opErr != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", cmd.Name, opErr))
		} else {
			imported++
		}
	}

	if imported > 0 {
		s.persist()
	}

	summary := fmt.Sprintf("Imported %d commands", imported)
	if skipped > 0 {
		summary += fmt.Sprintf(", skipped %d (already exist)", skipped)
	}
	if len(errors) > 0 {
		summary += fmt.Sprintf(", %d errors: %v", len(errors), errors)
	}

	log.Printf("Import from %s: %s", path, summary)
	return s.respondText(msg.ID, summary)
}

func (s *Server) handleExportConfig(msg *JSONRPCMessage, params ToolsCallParams) error {
	path, _ := params.Arguments["path"].(string)
	if path == "" {
		path = ".instant-mcp/commands.yaml"
	}

	cmds := s.registry.Snapshot()
	if len(cmds) == 0 {
		return s.respondError(msg.ID, "no commands to export")
	}

	file := importFile{Commands: cmds}

	data, err := yaml.Marshal(file)
	if err != nil {
		return s.respondError(msg.ID, fmt.Sprintf("failed to marshal YAML: %v", err))
	}

	// Add header comment
	header := "# instant-mcp commands\n# Generated by export_config\n# Import with: import_config(path: \"" + path + "\")\n\n"

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return s.respondError(msg.ID, fmt.Sprintf("failed to create directory: %v", err))
		}
	}

	if err := os.WriteFile(path, []byte(header+string(data)), 0644); err != nil {
		return s.respondError(msg.ID, fmt.Sprintf("failed to write file: %v", err))
	}

	// Sort command names for display
	names := make([]string, 0, len(cmds))
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)

	log.Printf("Exported %d commands to %s", len(cmds), path)
	return s.respondText(msg.ID, fmt.Sprintf("Exported %d commands to %s: %v", len(cmds), path, names))
}

// --- Response Helpers ---

func (s *Server) respondText(id any, text string) error {
	return s.transport.WriteResponse(id, ToolsCallResult{
		Content: []Content{{Type: "text", Text: text}},
	})
}

func (s *Server) respondError(id any, text string) error {
	return s.transport.WriteResponse(id, ToolsCallResult{
		Content: []Content{{Type: "text", Text: text}},
		IsError: true,
	})
}

// --- Argument Parsing ---

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
