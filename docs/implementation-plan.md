# instant-mcp Implementation Plan

## Overview

Build a dynamic MCP server in Go that allows agents to register custom commands at runtime. Implement tools-only interface with JSON persistence.

## Phases

### Phase 1: MCP Server Skeleton ✓ Ready to Start

**Goal:** Basic MCP server that responds to protocol requests

**Tasks:**
1. Initialize Go module (`go mod init`)
2. Create `main.go` with CLI flags (--state-file, --help)
3. Implement MCP stdio transport (read/write JSON-RPC)
4. Handle `initialize` request
5. Handle `tools/list` (return empty array initially)
6. Handle `tools/call` (stub implementation)
7. Manual test: connect via Claude Code, verify connection

**Acceptance:**
- Server starts without errors
- Claude Code sees server connected
- `tools/list` returns empty array
- Basic logging works

**Files Created:**
- `main.go`
- `server/server.go`
- `server/transport.go`

---

### Phase 2: Command Registry

**Goal:** In-memory command storage with basic CRUD

**Tasks:**
1. Define `Command` and `Arg` structs in `models/command.go`
2. Implement `Registry` in `server/registry.go`
   - `Add(cmd Command) error`
   - `Remove(name string) error`
   - `Get(name string) (Command, error)`
   - `List() []Command`
   - `Update(name string, cmd Command) error`
3. Add mutex for thread-safety
4. Unit tests for all registry operations

**Acceptance:**
- All registry operations work correctly
- Concurrent operations are safe (test with goroutines)
- Validation prevents duplicate names
- Tests cover edge cases (empty registry, missing commands)

**Files Created:**
- `models/command.go`
- `server/registry.go`
- `server/registry_test.go`

---

### Phase 3: Core CRUD Tools

**Goal:** Implement add_command, remove_command, list_commands, get_command

**Tasks:**
1. Create tool handler framework in `server/tools.go`
2. Implement `add_command` in `tools/add_command.go`
   - Parse input schema
   - Validate command
   - Add to registry
   - Return success/error
3. Implement `remove_command` in `tools/remove_command.go`
4. Implement `list_commands` in `tools/list_commands.go`
5. Implement `get_command` in `tools/get_command.go`
6. Wire up to `tools/call` handler in server
7. Integration tests for each tool

**Acceptance:**
- Agent can add a command via MCP
- Agent can list commands and see the added command
- Agent can get command details
- Agent can remove a command
- Validation errors return clear messages

**Files Created:**
- `tools/add_command.go`
- `tools/remove_command.go`
- `tools/list_commands.go`
- `tools/get_command.go`
- `tools/tools_test.go`

---

### Phase 4: Command Execution

**Goal:** Execute registered commands as subprocesses

**Tasks:**
1. Implement `Executor` in `server/executor.go`
   - `Execute(cmd Command, args map[string]interface{}) (output string, err error)`
   - Resolve executable path
   - Build command with arguments
   - Set timeout
   - Capture stdout/stderr
   - Handle subprocess errors
2. Dynamic tool registration from registry
   - For each command in registry, register as MCP tool
   - Map tool call to command execution
3. Handle async vs sync execution
4. Unit tests for executor
5. Integration test: register command → execute it → verify output

**Acceptance:**
- Registered command appears in tools/list
- Agent can call registered command
- Output is captured and returned
- Timeouts work correctly
- Errors are handled gracefully (missing executable, timeout, etc.)

**Files Created:**
- `server/executor.go`
- `server/executor_test.go`
- `server/dynamic_tools.go`

---

### Phase 5: Persistence Layer

**Goal:** Save/load registry from JSON file

**Tasks:**
1. Implement persistence in `server/persistence.go`
   - `Load(path string) (Registry, error)`
   - `Save(path string, reg Registry) error`
2. Define state file format (JSON with version field)
3. Handle missing file (return empty registry)
4. Handle corrupted file (log error, return empty, create backup)
5. Call Save() after each registry modification
6. Call Load() on server startup
7. Unit tests for persistence

**Acceptance:**
- Commands persist across server restarts
- Missing state file doesn't cause errors
- Corrupted file is backed up and replaced
- Default location works (`~/.instant-mcp/state.json`)
- Custom location via flag works

**Files Created:**
- `server/persistence.go`
- `server/persistence_test.go`

---

### Phase 6: batch_exec Tool

**Goal:** Atomic multi-command operations

**Tasks:**
1. Implement `batch_exec` in `tools/batch_exec.go`
   - Parse array of operations
   - If atomic=true, validate all before executing any
   - Execute operations in order
   - On failure (atomic=true), rollback all changes
   - On failure (atomic=false), continue and return partial results
2. Implement rollback mechanism in registry
3. Integration tests for batch operations
   - All succeed
   - Partial failure (atomic=false)
   - Failure with rollback (atomic=true)

**Acceptance:**
- Agent can register 5 commands in one tool call
- Atomic rollback works (5 operations, 3rd fails, registry unchanged)
- Partial success works (5 operations, 3rd fails, first 2 persist)
- Clear error messages indicate which operation failed

**Files Created:**
- `tools/batch_exec.go`
- `tools/batch_exec_test.go`

---

### Phase 7: Import/Export Tools

**Goal:** Git workflow support

**Tasks:**
1. Implement `export_config` in `tools/export_config.go`
   - Convert registry to YAML
   - Write to specified path (default: `.instant-mcp/commands.yaml`)
   - Include comments for human readability
2. Implement `import_config` in `tools/import_config.go`
   - Parse YAML or JSON
   - Validate commands
   - Add to registry (skip duplicates or use overwrite flag)
   - Return summary (N imported, M skipped, errors)
3. Add YAML library dependency (`gopkg.in/yaml.v3`)
4. Integration tests

**Acceptance:**
- Agent exports 10 commands to YAML
- YAML is human-readable with comments
- Agent imports YAML on fresh server, all 10 commands available
- Import handles duplicates correctly

**Files Created:**
- `tools/export_config.go`
- `tools/import_config.go`
- `tools/import_export_test.go`

---

### Phase 8: Help & Validation Tools

**Goal:** Self-documenting and user-friendly

**Tasks:**
1. Implement `help` tool in `tools/help.go`
   - Return markdown guide with examples
   - Cover all tools
   - Include common patterns
2. Implement `update_command` tool (missed in phase 3)
3. Improve error messages
   - Validation: suggest fixes (e.g., "name must be alphanumeric")
   - Execution: include stderr and exit code
4. Add logging throughout (use Go's `log` package)
5. Write user-facing documentation

**Acceptance:**
- Agent calls help and gets comprehensive guide
- All error messages are clear and actionable
- Logs provide audit trail of operations

**Files Created:**
- `tools/help.go`
- `tools/update_command.go`

---

### Phase 9: Polish & Release Prep

**Goal:** Production-ready

**Tasks:**
1. Add `--version` flag
2. Create build script for cross-platform binaries
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64)
3. Write CONTRIBUTING.md
4. Create GitHub release workflow
5. Create `.claude/skills/instant-mcp-usage/` skill
   - Teach agents how to use instant-mcp
   - Include examples
6. End-to-end testing
   - Fresh install → register command → execute → persist → restart → execute again
7. Performance testing (100+ commands)
8. Security review (ensure no privilege escalation)

**Acceptance:**
- Binaries build for all platforms
- Skill works (agent can use instant-mcp effectively)
- No memory leaks under load
- README, docs, and help are aligned

**Files Created:**
- `scripts/build.sh`
- `.claude/skills/instant-mcp-usage/skill.md`
- `.github/workflows/release.yml`
- `CONTRIBUTING.md`

---

## Dependencies

```
go.mod:
- gopkg.in/yaml.v3 (for YAML import/export)
- No other external dependencies (use stdlib)
```

## Testing Strategy

- **Unit tests** - Each package has `_test.go` files
- **Integration tests** - `integration_test.go` covers full workflows
- **Manual testing** - Use Claude Code to exercise all features
- **CI** - GitHub Actions runs tests on all PRs

## Development Environment Setup

```bash
# Clone repo
git clone <repo>
cd instant-mcp

# Install Go 1.21+
# (or use existing Go installation)

# Build
go build

# Run tests
go test ./...

# Run locally
./instant-mcp

# Connect Claude Code (add to MCP settings)
```

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| MCP protocol changes | Use official SDK once available, version carefully |
| Subprocess management bugs | Extensive testing with timeouts, signals |
| State file corruption | Backups, validation on load |
| Performance with many commands | Benchmark early, use profiling |
| Security vulnerabilities | Security review, clear trust boundary docs |

## Success Metrics

1. Agent can go from zero to working custom command in <1 minute
2. No crashes under normal usage (100+ hours runtime)
3. Clear error messages reduce support questions
4. Skill enables agents to use tool without human help
5. Positive feedback from early users

## Timeline Estimate

| Phase | Effort | Dependencies |
|-------|--------|--------------|
| 1. Server skeleton | 4 hours | None |
| 2. Registry | 3 hours | Phase 1 |
| 3. CRUD tools | 4 hours | Phase 2 |
| 4. Execution | 6 hours | Phase 3 |
| 5. Persistence | 3 hours | Phase 2 |
| 6. batch_exec | 4 hours | Phase 3, 5 |
| 7. Import/export | 3 hours | Phase 5 |
| 8. Help/validation | 2 hours | All above |
| 9. Polish | 4 hours | All above |
| **Total** | **33 hours** | |

Assumes: Experienced Go developer, focused work sessions, minimal blockers.

## Next Steps

1. Review this plan with stakeholders
2. Set up development environment
3. Begin Phase 1: MCP Server Skeleton
