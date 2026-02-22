# instant-mcp Project Guide

## What This Is

A dynamic MCP server that lets agents register custom commands at runtime by wrapping executables. Agents use MCP tools to add/remove/manage commands; a config file provides persistence.

## Architecture Decision: Tools-Only Interface

**Key insight from first-principles analysis:** Config file is persistence, not API.

- **Tools are the API** - add_command, remove_command, batch_exec, etc.
- **File is implementation detail** - JSON/YAML format can evolve without breaking agents
- **No file watching needed** - tools trigger updates synchronously
- **Single source of truth** - no dual-interface confusion

Agents never edit the config file directly. They use MCP tools exclusively.

## Language & Tech Stack

- **Go** - Static binary, excellent stdlib, cross-platform
- **MCP Protocol** - Stdio-based JSON-RPC
- **Persistence** - JSON file (can migrate to SQLite later)
- **File watching** - Not needed (tools-only interface)

## Project Structure

```
instant-mcp/
├── main.go                 # Entry point, CLI flags
├── server/
│   ├── server.go          # MCP server implementation
│   ├── registry.go        # Command registry (in-memory)
│   ├── persistence.go     # State file I/O
│   └── executor.go        # Command execution logic
├── tools/
│   ├── add_command.go     # Add single command
│   ├── remove_command.go  # Remove command
│   ├── batch_exec.go      # Atomic multi-command operations
│   ├── list_commands.go   # List all registered commands
│   ├── get_command.go     # Get command details
│   ├── import_config.go   # Bulk import from file
│   ├── export_config.go   # Export for git/version control
│   └── help.go            # Usage guide
├── models/
│   └── command.go         # Command data structures
├── docs/
│   ├── requirements.md    # Detailed requirements
│   └── implementation-plan.md
├── .claude/
│   └── skills/
│       └── instant-mcp-usage/  # Agent skill for using this
└── README.md
```

## Core Data Model

```go
type Command struct {
    Name        string            `json:"name"`
    Exec        string            `json:"exec"`
    Args        map[string]Arg    `json:"args"`
    Description string            `json:"description"`
    Async       bool              `json:"async"`
    Timeout     string            `json:"timeout"` // "30s", "5m"
}

type Arg struct {
    Type        string `json:"type"`        // "string", "number", "boolean"
    Description string `json:"description"`
    Required    bool   `json:"required"`
}
```

## State File Location

Default: `~/.instant-mcp/state.json`

Override with: `--state-file` flag or `INSTANT_MCP_STATE` env var

## Security Model

**Trust boundary:** This tool allows arbitrary code execution.

- Agents must be trusted
- Executables are run with server's permissions
- No sandboxing (by design - flexibility over safety)
- Document prominently in README

## Error Handling Philosophy

- **Validate early** - Tools validate before persistence
- **Helpful messages** - "executable './foo.sh' not found at /Users/hays/Projects/demo/foo.sh"
- **Graceful degradation** - If state file is corrupted, start with empty registry + log error
- **No silent failures** - Always return error or success clearly

## Testing Strategy

- Unit tests for registry operations
- Integration tests for tool execution
- Mock filesystem for persistence tests
- Test invalid inputs, edge cases, concurrent operations

## Code Style

- Idiomatic Go (gofmt, golint)
- Prefer stdlib over dependencies
- Clear error messages
- Comments for exported functions
- Keep functions small and focused

## Key Design Decisions (From First Principles)

1. **Executables as primitive** - Maximum flexibility, language-agnostic
2. **Tools-only interface** - Single API surface, config is implementation detail
3. **Atomic batch operations** - Reduce round-trips, enable complex setup
4. **No file watching** - Tools trigger updates directly, simpler implementation
5. **Import/export for git workflow** - Best of both worlds (tools + version control)

## Non-Goals

- ❌ Context isolation / one-shot LLM calls (use Task tool instead)
- ❌ Sandboxing / security restrictions (trust model)
- ❌ Remote execution (local only)
- ❌ Complex argument validation (basic types only)

## Future Enhancements (Post-MVP)

- Environment variable support in commands
- Working directory specification per command
- Stdin/stdout streaming for long-running commands
- SQLite persistence (migrate from JSON)
- Command groups/namespaces
