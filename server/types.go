package server

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema describes the JSON Schema for tool input
type InputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// ToolsCallParams is the params for a tools/call request
type ToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolsCallResult is the result for a tools/call response
type ToolsCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents a content block in a tool result
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolHandler is the function signature for built-in tool handlers
type toolHandler func(msg *JSONRPCMessage, params ToolsCallParams) error
