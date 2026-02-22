# instant-mcp Requirements

## Overview

A Model Context Protocol (MCP) server that allows agents to dynamically register custom commands at runtime by wrapping executables. The server provides MCP tools for managing commands, with automatic persistence.

## Functional Requirements

### FR1: MCP Server

- **FR1.1** - Implement MCP protocol over stdio (JSON-RPC)
- **FR1.2** - Respond to `initialize` request with server capabilities
- **FR1.3** - Handle `tools/list` to enumerate available tools
- **FR1.4** - Handle `tools/call` to execute tools
- **FR1.5** - Return proper error responses per MCP spec

### FR2: Command Registry (CRUD)

- **FR2.1** - Maintain in-memory registry of registered commands
- **FR2.2** - Support adding commands with full metadata
- **FR2.3** - Support removing commands by name
- **FR2.4** - Support updating existing commands
- **FR2.5** - Support listing all registered commands
- **FR2.6** - Support retrieving single command details

### FR3: Command Execution

- **FR3.1** - Execute registered commands as subprocesses
- **FR3.2** - Pass arguments from MCP tool call to executable
- **FR3.3** - Support synchronous (blocking) execution
- **FR3.4** - Support asynchronous (non-blocking) execution
- **FR3.5** - Enforce configurable timeouts
- **FR3.6** - Capture stdout/stderr from executed commands
- **FR3.7** - Return exit code and output to agent

### FR4: Built-in Tools

#### FR4.1: add_command
- Input: name, exec, args (optional), description, async, timeout
- Validation: name unique, exec path exists, timeout valid format
- Output: success confirmation or error
- Side effect: Adds to registry, persists to file

#### FR4.2: remove_command
- Input: name
- Validation: command exists
- Output: success confirmation or error
- Side effect: Removes from registry, persists to file

#### FR4.3: update_command
- Input: name, fields to update
- Validation: command exists, updates are valid
- Output: success confirmation or error
- Side effect: Updates registry, persists to file

#### FR4.4: list_commands
- Input: none
- Output: Array of command summaries (name, description, async)
- No side effects

#### FR4.5: get_command
- Input: name
- Validation: command exists
- Output: Full command details
- No side effects

#### FR4.6: batch_exec
- Input: array of operations, atomic flag
- Operations: add_command, remove_command, update_command
- Validation: all operations valid before executing any (if atomic=true)
- Output: array of results or rollback on failure
- Side effect: Multiple registry changes, single persist

#### FR4.7: import_config
- Input: file path (YAML or JSON)
- Validation: file exists, valid format, commands valid
- Output: count of commands imported, list of errors
- Side effect: Adds all commands to registry, persists

#### FR4.8: export_config
- Input: output path (optional, defaults to .instant-mcp/commands.yaml)
- Output: success confirmation with path
- Side effect: Writes YAML file with current commands

#### FR4.9: help
- Input: none
- Output: Usage guide with examples
- No side effects

### FR5: Persistence

- **FR5.1** - Store command registry in JSON file
- **FR5.2** - Load registry from file on server startup
- **FR5.3** - Persist registry after each modification
- **FR5.4** - Handle missing state file (start with empty registry)
- **FR5.5** - Handle corrupted state file (log error, start with empty registry)
- **FR5.6** - Support configurable state file location

### FR6: Validation

- **FR6.1** - Validate command names (alphanumeric + underscore, no duplicates)
- **FR6.2** - Validate executable paths (file exists and is executable)
- **FR6.3** - Validate argument specifications (valid types, required flags)
- **FR6.4** - Validate timeout format ("30s", "5m", etc.)
- **FR6.5** - Return clear error messages for validation failures

## Non-Functional Requirements

### NFR1: Performance

- **NFR1.1** - Command registration completes in <100ms
- **NFR1.2** - Persistence writes complete in <50ms
- **NFR1.3** - Support 100+ registered commands without degradation

### NFR2: Reliability

- **NFR2.1** - Graceful handling of subprocess failures
- **NFR2.2** - No data loss on crashes (state persisted after each change)
- **NFR2.3** - Atomic batch operations (all succeed or all fail)

### NFR3: Usability

- **NFR3.1** - Clear, actionable error messages
- **NFR3.2** - Self-documenting (help tool provides full guide)
- **NFR3.3** - Examples in tool descriptions

### NFR4: Security

- **NFR4.1** - Document trust boundary (arbitrary code execution)
- **NFR4.2** - No privilege escalation beyond server process
- **NFR4.3** - Log command executions for audit trail

### NFR5: Maintainability

- **NFR5.1** - Idiomatic Go code (gofmt, golint clean)
- **NFR5.2** - Unit test coverage >80%
- **NFR5.3** - Clear separation of concerns (registry, persistence, execution)

## Data Model

### Command Structure

```go
type Command struct {
    Name        string            `json:"name"`        // Unique identifier
    Exec        string            `json:"exec"`        // Path to executable
    Args        map[string]Arg    `json:"args"`        // Argument specifications
    Description string            `json:"description"` // Help text
    Async       bool              `json:"async"`       // Blocking or non-blocking
    Timeout     string            `json:"timeout"`     // "30s", "5m", etc.
}

type Arg struct {
    Type        string `json:"type"`        // "string", "number", "boolean"
    Description string `json:"description"` // Help text for this arg
    Required    bool   `json:"required"`    // Must be provided?
}
```

### State File Format

```json
{
  "version": "1.0",
  "commands": {
    "analyze_logs": {
      "name": "analyze_logs",
      "exec": "./scripts/analyze-logs.sh",
      "args": {
        "log_file": {
          "type": "string",
          "description": "Path to log file",
          "required": true
        }
      },
      "description": "Analyzes application logs for errors",
      "async": false,
      "timeout": "30s"
    }
  }
}
```

## MCP Protocol Requirements

### Tool Definition Format

Each registered command becomes an MCP tool:

```json
{
  "name": "analyze_logs",
  "description": "Analyzes application logs for errors",
  "inputSchema": {
    "type": "object",
    "properties": {
      "log_file": {
        "type": "string",
        "description": "Path to log file"
      }
    },
    "required": ["log_file"]
  }
}
```

### Error Responses

Per MCP spec, errors include:
- `code`: Error code (integer)
- `message`: Human-readable error message
- `data`: Optional additional context

## Edge Cases

### EC1: Concurrent Operations
- Multiple tool calls in quick succession
- Solution: Mutex around registry modifications

### EC2: Long-Running Commands
- Command exceeds timeout
- Solution: Kill subprocess, return timeout error

### EC3: Missing Executable
- Command registered, but script deleted
- Solution: Validation on add, runtime error on execute

### EC4: Corrupted State File
- JSON parse error on load
- Solution: Log error, start with empty registry, create backup

### EC5: Duplicate Command Names
- Adding command with existing name
- Solution: Return error, suggest using update_command instead

### EC6: Batch Operation Partial Failure
- 3 of 5 operations succeed, 2 fail (atomic=false)
- Solution: Return array of results with success/failure per operation

### EC7: Import Conflicts
- Import file contains names that already exist
- Solution: Skip duplicates by default, add `overwrite` flag option

## Out of Scope (Explicitly Not Included)

- Remote command execution (local only)
- Command chaining / pipelines (agent handles this)
- Argument validation beyond type checking (trust executable)
- Sandboxing / permission restrictions (trust model)
- Streaming output for long commands (future enhancement)
- Authentication / authorization (MCP client handles this)
- Multi-user support (single registry per server instance)

## Success Criteria

1. Agent can register a command and use it within 2 tool calls
2. batch_exec successfully registers 10 commands atomically
3. Server survives 1000+ command executions without memory leaks
4. Export → import workflow preserves all command metadata
5. Clear error messages for all validation failures
6. Zero configuration required (works with defaults out of box)
