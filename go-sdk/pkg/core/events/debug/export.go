package debug

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ExportSession exports a debugging session to the specified format
func (d *ValidationDebugger) ExportSession(sessionID string, format string) (string, error) {
	session := d.GetSession(sessionID)
	if session == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	filename := fmt.Sprintf("%s_%s.%s", sessionID, time.Now().Format("20060102_150405"), format)
	filepath := filepath.Join(d.outputDir, filename)

	switch format {
	case "json":
		return d.exportToJSON(session, filepath)
	case "csv":
		return d.exportToCSV(session, filepath)
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportToJSON exports a session to JSON format
func (d *ValidationDebugger) exportToJSON(session *ValidationSession, filepath string) (string, error) {
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(session); err != nil {
		return "", fmt.Errorf("failed to encode JSON: %w", err)
	}

	d.logger.WithField("file", filepath).Info("Exported session to JSON")
	return filepath, nil
}

// exportToCSV exports a session to CSV format
func (d *ValidationDebugger) exportToCSV(session *ValidationSession, filepath string) (string, error) {
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Index", "Timestamp", "EventType", "RuleID", "Duration", "Valid", "Errors", "Warnings",
		"MemoryAlloc", "MemoryObjects", "ExecutionError",
	}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, entry := range session.Events {
		for _, exec := range entry.Executions {
			row := []string{
				strconv.Itoa(entry.Index),
				entry.Timestamp.Format(time.RFC3339),
				string(entry.Event.Type()),
				exec.RuleID,
				exec.Duration.String(),
				strconv.FormatBool(exec.Result != nil && exec.Result.IsValid),
				strconv.Itoa(len(exec.Result.Errors)),
				strconv.Itoa(len(exec.Result.Warnings)),
				strconv.FormatUint(exec.MemoryAfter.Alloc-exec.MemoryBefore.Alloc, 10),
				strconv.FormatUint(exec.MemoryAfter.HeapObjects-exec.MemoryBefore.HeapObjects, 10),
				exec.Error,
			}

			if err := writer.Write(row); err != nil {
				return "", fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}

	d.logger.WithField("file", filepath).Info("Exported session to CSV")
	return filepath, nil
}
