package server

import (
	"encoding/json"
	"fmt"
	"log"
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
			Tools: map[string]any{
				"listChanged": true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}

	return s.transport.WriteResponse(msg.ID, result)
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
