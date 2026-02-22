package server

import (
	"encoding/json"
	"fmt"
)

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
