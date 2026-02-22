package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Transport handles stdio-based JSON-RPC communication
type Transport struct {
	reader *bufio.Reader
	writer io.Writer
}

// NewTransport creates a new stdio transport
func NewTransport() *Transport {
	return &Transport{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}
}

// ReadMessage reads and parses a JSON-RPC message from stdin
func (t *Transport) ReadMessage() (*JSONRPCMessage, error) {
	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var msg JSONRPCMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	log.Printf("← %s id=%v", msg.Method, msg.ID)
	return &msg, nil
}

// WriteMessage writes a JSON-RPC message to stdout
func (t *Transport) WriteMessage(msg *JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC message: %w", err)
	}

	data = append(data, '\n')
	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if msg.Error != nil {
		log.Printf("→ error id=%v: %s", msg.ID, msg.Error.Message)
	} else {
		log.Printf("→ result id=%v", msg.ID)
	}
	return nil
}

// WriteResponse writes a JSON-RPC response
func (t *Transport) WriteResponse(id any, result any) error {
	return t.WriteMessage(&JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

// WriteError writes a JSON-RPC error response
func (t *Transport) WriteError(id any, code int, message string, data any) error {
	return t.WriteMessage(&JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}
