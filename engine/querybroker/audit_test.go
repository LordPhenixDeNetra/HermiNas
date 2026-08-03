package querybroker

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditRecordAppendsJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	log, err := OpenAuditLog(path)
	if err != nil {
		t.Fatalf("OpenAuditLog failed: %v", err)
	}
	defer log.Close()

	log.Record(AuditEntry{UserID: "alice", SQL: "SELECT 1", StartedAt: time.Now(), RowCount: 1, Success: true})
	log.Record(AuditEntry{UserID: "alice", SQL: "SELECT bad", StartedAt: time.Now(), Success: false, Error: "boom"})

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	var entries []AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("decode audit line: %v", err)
		}
		entries = append(entries, e)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(entries))
	}
	if entries[0].SQL != "SELECT 1" || !entries[0].Success {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].SQL != "SELECT bad" || entries[1].Success || entries[1].Error != "boom" {
		t.Errorf("unexpected second entry: %+v", entries[1])
	}
}

func TestAuditIsAppendOnlyAcrossReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")

	log1, err := OpenAuditLog(path)
	if err != nil {
		t.Fatalf("OpenAuditLog failed: %v", err)
	}
	log1.Record(AuditEntry{UserID: "alice", SQL: "SELECT 1", StartedAt: time.Now(), Success: true})
	log1.Close()

	log2, err := OpenAuditLog(path)
	if err != nil {
		t.Fatalf("re-OpenAuditLog failed: %v", err)
	}
	defer log2.Close()
	log2.Record(AuditEntry{UserID: "alice", SQL: "SELECT 2", StartedAt: time.Now(), Success: true})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	lineCount := 0
	for _, b := range data {
		if b == '\n' {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Fatalf("expected 2 lines after reopening and appending, got %d", lineCount)
	}
}
