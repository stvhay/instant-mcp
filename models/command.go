package models

// Command represents a registered command that can be executed as an MCP tool
type Command struct {
	Name        string         `json:"name"`
	Exec        string         `json:"exec"`
	Args        map[string]Arg `json:"args,omitempty"`
	Description string         `json:"description,omitempty"`
	Async       bool           `json:"async,omitempty"`
	Timeout     string         `json:"timeout,omitempty"` // "30s", "5m", etc.
}

// Arg represents a command argument specification
type Arg struct {
	Type        string `json:"type"`                  // "string", "number", "boolean"
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}
