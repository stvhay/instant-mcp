# instant-mcp

**Dynamic MCP server that lets agents register custom commands at runtime.**

Agents can extend their own capabilities by wrapping executables as MCP tools - no server restarts, no configuration files to edit, just use MCP tools to add commands on the fly.

## ⚠️ Security Warning

**This tool allows arbitrary code execution.** Only use with trusted agents and in environments where you trust the code being executed. Commands run with the same permissions as the instant-mcp server process.

## What Problem Does This Solve?

Creating custom MCP servers requires:
- Writing boilerplate (Node.js/Python setup)
- Managing dependencies
- Configuring Claude Code to connect
- Restarting servers for changes

**instant-mcp eliminates this friction.** Agents can:
1. Write a script (any language)
2. Call `add_command` to register it as an MCP tool
3. Immediately use the new tool

## Quick Start

### Installation

```bash
# Install from source
go install github.com/yourusername/instant-mcp@latest

# Or download binary from releases
curl -L https://github.com/yourusername/instant-mcp/releases/latest/download/instant-mcp-$(uname -s)-$(uname -m) -o instant-mcp
chmod +x instant-mcp
```

### Configure Claude Code

Add to your MCP settings:

```json
{
  "mcpServers": {
    "instant-mcp": {
      "command": "instant-mcp",
      "args": []
    }
  }
}
```

### Basic Usage

**Agent registers a command:**

```
Use the add_command tool:
{
  "name": "analyze_logs",
  "exec": "./scripts/analyze-logs.sh",
  "args": {
    "log_file": {
      "type": "string",
      "description": "Path to log file",
      "required": true
    }
  },
  "description": "Analyzes application logs for errors"
}
```

**Command is now available as MCP tool:**

```
Use the analyze_logs tool:
{
  "log_file": "/var/log/app.log"
}
```

## Core Tools

| Tool | Purpose |
|------|---------|
| `help` | Get usage guide and examples |
| `add_command` | Register a new command |
| `remove_command` | Unregister a command |
| `list_commands` | Show all registered commands |
| `get_command` | Get details of specific command |
| `batch_exec` | Register multiple commands atomically |
| `import_config` | Bulk import commands from YAML/JSON |
| `export_config` | Export commands for version control |

## Examples

### Single Command

```json
{
  "name": "git_summary",
  "exec": "git",
  "args": {
    "repo_path": {"type": "string", "required": true}
  },
  "description": "Get git repository summary",
  "async": false,
  "timeout": "10s"
}
```

### Batch Setup

```json
{
  "commands": [
    {
      "operation": "add_command",
      "params": {
        "name": "lint",
        "exec": "./scripts/lint.sh"
      }
    },
    {
      "operation": "add_command",
      "params": {
        "name": "test",
        "exec": "./scripts/test.sh"
      }
    },
    {
      "operation": "add_command",
      "params": {
        "name": "deploy",
        "exec": "./scripts/deploy.sh"
      }
    }
  ],
  "atomic": true
}
```

### Version Control Workflow

```bash
# Agent exports current commands
export_config → .instant-mcp/commands.yaml

# Commit to git
git add .instant-mcp/commands.yaml
git commit -m "Add analysis commands"

# On new machine, import
import_config from .instant-mcp/commands.yaml
```

## How It Works

```
┌─────────────────────────────────┐
│   Claude Code (MCP Client)      │
└────────────┬────────────────────┘
             │ MCP Protocol (stdio)
             │
┌────────────▼────────────────────┐
│   instant-mcp server             │
│                                  │
│   Built-in tools:                │
│   • add_command                  │
│   • remove_command               │
│   • batch_exec                   │
│   • list_commands                │
│                                  │
│   Dynamic tools:                 │
│   • (registered by agent)        │
└────────────┬────────────────────┘
             │
      ┌──────▼──────┐
      │ state.json  │ (persistence)
      └─────────────┘
             │
      ┌──────▼──────┐
      │ Your scripts │ (executables)
      └─────────────┘
```

## Configuration

### State File Location

Default: `~/.instant-mcp/state.json`

Override:
- Flag: `instant-mcp --state-file /path/to/state.json`
- Env: `INSTANT_MCP_STATE=/path/to/state.json instant-mcp`

### Command Search Path

Executables are resolved relative to:
1. Absolute paths (start with `/`)
2. Relative to current working directory
3. Paths in `$PATH`

## Philosophy

**Tools are the API, files are persistence.**

Agents use MCP tools exclusively to manage commands. The state file is an implementation detail that agents never edit directly. This keeps the interface clean and allows the persistence format to evolve.

## Use Cases

- **Project-specific utilities** - Register build/test/deploy scripts per project
- **Data analysis pipelines** - Chain commands for log parsing, data transformation
- **Custom integrations** - Wrap APIs, databases, external tools
- **Rapid prototyping** - Test tool ideas without building full MCP servers

## Development

```bash
# Clone and build
git clone https://github.com/yourusername/instant-mcp
cd instant-mcp
go build

# Run tests
go test ./...

# Run locally
./instant-mcp
```

## Roadmap

- [x] Core CRUD operations (add, remove, list, get)
- [x] Command execution with timeout
- [x] Batch operations
- [x] Import/export for git workflow
- [ ] Environment variable support
- [ ] Working directory per command
- [ ] Stdin/stdout streaming
- [ ] SQLite persistence option

## Contributing

Issues and PRs welcome! Please read CONTRIBUTING.md first.

## License

MIT

## Credits

Built with the [MCP Protocol](https://modelcontextprotocol.io/) by Anthropic.
