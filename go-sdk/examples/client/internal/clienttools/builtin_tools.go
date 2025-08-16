package clienttools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// NewShellCommandTool creates a tool that executes shell commands
func NewShellCommandTool() ToolFunc {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		command, ok := params["command"].(string)
		if !ok {
			return nil, fmt.Errorf("command parameter is required and must be a string")
		}

		workingDir := "."
		if wd, ok := params["working_dir"].(string); ok {
			workingDir = wd
		}

		timeout := 30
		if t, ok := params["timeout"]; ok {
			switch v := t.(type) {
			case int:
				timeout = v
			case float64:
				timeout = int(v)
			case string:
				if parsed, err := strconv.Atoi(v); err == nil {
					timeout = parsed
				}
			}
		}

		// Create command with timeout
		cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Determine shell based on OS
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(cmdCtx, "cmd", "/C", command)
		} else {
			cmd = exec.CommandContext(cmdCtx, "sh", "-c", command)
		}

		cmd.Dir = workingDir

		// Capture output
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute command
		err := cmd.Run()

		result := map[string]interface{}{
			"stdout":    stdout.String(),
			"stderr":    stderr.String(),
			"exit_code": 0,
		}

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result["exit_code"] = exitErr.ExitCode()
			} else {
				return nil, fmt.Errorf("command execution failed: %w", err)
			}
		}

		return result, nil
	}
}

// NewFileReaderTool creates a tool that reads local files
func NewFileReaderTool() ToolFunc {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		path, ok := params["path"].(string)
		if !ok {
			return nil, fmt.Errorf("path parameter is required and must be a string")
		}

		encoding := "utf-8"
		if enc, ok := params["encoding"].(string); ok {
			encoding = enc
		}

		// Check if file exists
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file not found: %s", path)
			}
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}

		// Check if it's a regular file
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("path is not a regular file: %s", path)
		}

		// Check file size (limit to 10MB)
		maxSize := int64(10 * 1024 * 1024)
		if info.Size() > maxSize {
			return nil, fmt.Errorf("file too large: %d bytes (max %d bytes)", info.Size(), maxSize)
		}

		// Read file contents
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		result := map[string]interface{}{
			"path": path,
			"size": info.Size(),
		}

		switch encoding {
		case "base64":
			result["content"] = base64.StdEncoding.EncodeToString(data)
			result["encoding"] = "base64"
		case "utf-8", "ascii":
			result["content"] = string(data)
			result["encoding"] = encoding
		default:
			return nil, fmt.Errorf("unsupported encoding: %s", encoding)
		}

		return result, nil
	}
}

// NewCalculatorTool creates a tool that performs mathematical calculations
func NewCalculatorTool() ToolFunc {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		expression, ok := params["expression"].(string)
		if !ok {
			return nil, fmt.Errorf("expression parameter is required and must be a string")
		}

		// Simple expression evaluator
		// In production, use a proper expression parser
		result, err := evaluateExpression(expression)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression: %w", err)
		}

		return map[string]interface{}{
			"expression": expression,
			"result":     result,
		}, nil
	}
}

// NewSystemInfoTool creates a tool that retrieves system information
func NewSystemInfoTool() ToolFunc {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		infoType := "all"
		if it, ok := params["info_type"].(string); ok {
			infoType = it
		}

		result := make(map[string]interface{})

		switch infoType {
		case "os", "all":
			result["os"] = map[string]interface{}{
				"platform": runtime.GOOS,
				"arch":     runtime.GOARCH,
				"version":  runtime.Version(),
				"cpus":     runtime.NumCPU(),
			}
			if infoType == "os" {
				break
			}
			fallthrough

		case "cpu":
			if infoType == "cpu" || infoType == "all" {
				result["cpu"] = map[string]interface{}{
					"cores":       runtime.NumCPU(),
					"goroutines":  runtime.NumGoroutine(),
					"gomaxprocs":  runtime.GOMAXPROCS(0),
				}
			}
			if infoType == "cpu" {
				break
			}
			fallthrough

		case "memory":
			if infoType == "memory" || infoType == "all" {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				result["memory"] = map[string]interface{}{
					"alloc":       m.Alloc,
					"total_alloc": m.TotalAlloc,
					"sys":         m.Sys,
					"num_gc":      m.NumGC,
				}
			}
			if infoType == "memory" {
				break
			}
			fallthrough

		case "disk":
			if infoType == "disk" || infoType == "all" {
				// Get current working directory disk usage
				wd, _ := os.Getwd()
				result["disk"] = map[string]interface{}{
					"working_dir": wd,
					// In production, use proper disk usage API
				}
			}
			if infoType == "disk" {
				break
			}
			fallthrough

		case "network":
			if infoType == "network" || infoType == "all" {
				hostname, _ := os.Hostname()
				result["network"] = map[string]interface{}{
					"hostname": hostname,
					// In production, use proper network info API
				}
			}

		default:
			return nil, fmt.Errorf("unknown info_type: %s", infoType)
		}

		return result, nil
	}
}

// evaluateExpression is a simple expression evaluator
// In production, use a proper expression parser like govaluate
func evaluateExpression(expr string) (float64, error) {
	// Remove spaces
	expr = strings.ReplaceAll(expr, " ", "")

	// Very basic calculator - only supports simple operations
	// This is just for demonstration - use a proper parser in production
	
	// Try to parse as a simple number first
	if val, err := strconv.ParseFloat(expr, 64); err == nil {
		return val, nil
	}

	// Support basic operations
	operators := []string{"+", "-", "*", "/"}
	for _, op := range operators {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) != 2 {
				continue
			}

			left, err := strconv.ParseFloat(parts[0], 64)
			if err != nil {
				continue
			}

			right, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				continue
			}

			switch op {
			case "+":
				return left + right, nil
			case "-":
				return left - right, nil
			case "*":
				return left * right, nil
			case "/":
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return left / right, nil
			}
		}
	}

	return 0, fmt.Errorf("unable to evaluate expression: %s", expr)
}