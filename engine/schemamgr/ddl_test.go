package schemamgr

import (
	"strings"
	"testing"
)

func TestGenerateDDLBasicShape(t *testing.T) {
	d := Dataset{
		Name: "app_logs",
		Columns: []Column{
			{Name: "message", Type: "String"},
			{Name: "level", Type: "String", Nullable: true},
		},
		OrderBy:           []string{"level"},
		PartitionByColumn: "received_at",
		TTLDays:           30,
	}

	ddl := GenerateDDL(d)

	wantContains := []string{
		"CREATE TABLE IF NOT EXISTS `app_logs`",
		"`message` String",
		"`level` Nullable(String)",
		"ENGINE = MergeTree()",
		"PARTITION BY toYYYYMM(`received_at`)",
		"ORDER BY (`level`)",
		"TTL `received_at` + INTERVAL 30 DAY",
	}
	for _, want := range wantContains {
		if !strings.Contains(ddl, want) {
			t.Errorf("DDL missing %q in:\n%s", want, ddl)
		}
	}
}

func TestGenerateDDLWithoutOrderByUsesTuple(t *testing.T) {
	d := Dataset{Name: "raw", Columns: []Column{{Name: "a", Type: "String"}}}
	ddl := GenerateDDL(d)
	if !strings.Contains(ddl, "ORDER BY tuple()") {
		t.Errorf("expected ORDER BY tuple() fallback, got:\n%s", ddl)
	}
}

func TestGenerateDDLWithoutPartitionOmitsPartitionClause(t *testing.T) {
	d := Dataset{Name: "raw", Columns: []Column{{Name: "a", Type: "String"}}}
	ddl := GenerateDDL(d)
	if strings.Contains(ddl, "PARTITION BY") {
		t.Errorf("expected no PARTITION BY clause, got:\n%s", ddl)
	}
}

func TestGenerateDDLQuotesIdentifiersWithBackticks(t *testing.T) {
	d := Dataset{Name: "order", Columns: []Column{{Name: "select", Type: "String"}}}
	ddl := GenerateDDL(d)
	if !strings.Contains(ddl, "`order`") || !strings.Contains(ddl, "`select`") {
		t.Errorf("expected SQL-keyword-like identifiers to be quoted, got:\n%s", ddl)
	}
}
