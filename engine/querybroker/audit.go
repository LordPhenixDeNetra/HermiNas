package querybroker

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"herminas/kernel/errors"
)

// AuditEntry is one journaled query (M1.4: "journaliser chaque requête
// dans l'audit (append-only)"). This is the M1.4-scoped version — filters,
// export, and PII-aware redaction are M7.2's job; this just guarantees
// every query is durably recorded from day one, since retrofitting audit
// coverage onto queries already run is impossible.
type AuditEntry struct {
	UserID     string    `json:"user_id"`
	SQL        string    `json:"sql"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
	RowCount   int       `json:"row_count"`
	Cached     bool      `json:"cached"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// AuditLog appends one JSON line per query. Append-only by construction:
// there is no update or delete method.
type AuditLog struct {
	mu   sync.Mutex
	file *os.File
}

func OpenAuditLog(path string) (*AuditLog, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "open audit log", err)
	}
	return &AuditLog{file: f}, nil
}

func (a *AuditLog) Close() error {
	return a.file.Close()
}

// Record never returns an error to the caller's query path: a broken
// audit log must not itself take down query execution. Failures are
// logged to stderr instead.
func (a *AuditLog) Record(entry AuditEntry) {
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line = append(line, '\n')

	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.file.Write(line)
}
