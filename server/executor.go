package server

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hays/instant-mcp/models"
)

// Execute runs a registered command with the given arguments
func Execute(cmd models.Command, args map[string]any) (string, error) {
	// Validate required args
	for argName, argSpec := range cmd.Args {
		if argSpec.Required {
			if _, ok := args[argName]; !ok {
				return "", fmt.Errorf("missing required argument: %s", argName)
			}
		}
	}

	// Build command line arguments
	execArgs := buildArgs(cmd, args)

	// Parse timeout
	timeout := 120 * time.Second
	if cmd.Timeout != "" {
		parsed, err := parseTimeout(cmd.Timeout)
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = parsed
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Resolve executable
	execPath, err := resolveExec(cmd.Exec)
	if err != nil {
		return "", err
	}

	// Run
	c := exec.CommandContext(ctx, execPath, execArgs...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err = c.Run()

	output := stdout.String()
	if errOut := stderr.String(); errOut != "" {
		if output != "" {
			output += "\n"
		}
		output += "stderr: " + errOut
	}

	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %s", cmd.Timeout)
	}

	if err != nil {
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

func buildArgs(cmd models.Command, args map[string]any) []string {
	var result []string
	for argName, val := range args {
		// Only include args that are defined in the command spec
		if _, defined := cmd.Args[argName]; !defined {
			continue
		}
		result = append(result, argToString(val))
		_ = argName // arg name not used as flag, just positional for now
	}
	return result
}

func argToString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func resolveExec(path string) (string, error) {
	// Absolute path
	if strings.HasPrefix(path, "/") {
		return path, nil
	}

	// Try PATH lookup
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("executable %q not found: %w", path, err)
	}
	return resolved, nil
}

func parseTimeout(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid timeout format: %q", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout number: %q", s)
	}

	switch unit {
	case 's':
		return time.Duration(num) * time.Second, nil
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid timeout unit %q (use s, m, or h)", string(unit))
	}
}
