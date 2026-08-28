package wiki

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/config"
)

// Operation identifies the kind of log entry.
type Operation string

const (
	OpInit      Operation = "init"
	OpIngest    Operation = "ingest"
	OpRemember  Operation = "remember"
	OpRecall    Operation = "recall"
	OpLint      Operation = "lint"
	OpReconcile Operation = "reconcile"
	OpReindex   Operation = "reindex"
	OpQuery     Operation = "query"
)

// AppendLog records an operation in the append-only log.md.
func AppendLog(paths *config.Paths, op Operation, subject string, details string) error {
	if err := os.MkdirAll(paths.WikiDir, 0o755); err != nil {
		return fmt.Errorf("create wiki directory: %w", err)
	}

	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	line := fmt.Sprintf("## [%s] %s | %s\n\n%s\n\n", ts, op, subject, details)

	f, err := os.OpenFile(paths.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write log: %w", err)
	}
	return nil
}

// ReadLogTail returns the last n log entries (crude parse by "## [" delimiter).
func ReadLogTail(paths *config.Paths, n int) ([]string, error) {
	data, err := os.ReadFile(paths.LogFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read log: %w", err)
	}

	var entries []string
	for _, raw := range strings.Split(string(data), "## [") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		entries = append(entries, "## ["+raw)
	}

	if len(entries) > n {
		entries = entries[len(entries)-n:]
	}
	return entries, nil
}
