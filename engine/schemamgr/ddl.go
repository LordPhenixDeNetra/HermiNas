package schemamgr

import (
	"fmt"
	"strings"
)

// GenerateDDL turns a validated Dataset into a `CREATE TABLE` statement for
// the embedded ClickHouse engine (M1.3). Caller must have already run
// Dataset.Validate — this does not re-check consistency.
func GenerateDDL(d Dataset) string {
	var b strings.Builder

	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s (\n", quoteIdent(d.Name))
	for i, col := range d.Columns {
		colType := col.Type
		if col.Nullable {
			colType = fmt.Sprintf("Nullable(%s)", colType)
		}
		sep := ","
		if i == len(d.Columns)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "    %s %s%s\n", quoteIdent(col.Name), colType, sep)
	}
	b.WriteString(") ENGINE = MergeTree()\n")

	if d.PartitionByColumn != "" {
		fmt.Fprintf(&b, "PARTITION BY toYYYYMM(%s)\n", quoteIdent(d.PartitionByColumn))
	}

	if len(d.OrderBy) > 0 {
		quoted := make([]string, len(d.OrderBy))
		for i, col := range d.OrderBy {
			quoted[i] = quoteIdent(col)
		}
		fmt.Fprintf(&b, "ORDER BY (%s)\n", strings.Join(quoted, ", "))
	} else {
		b.WriteString("ORDER BY tuple()\n")
	}

	if d.TTLDays > 0 {
		fmt.Fprintf(&b, "TTL %s + INTERVAL %d DAY\n", quoteIdent(d.PartitionByColumn), d.TTLDays)
	}

	return strings.TrimRight(b.String(), "\n")
}

// GenerateAlterDDL turns newly-added columns into an `ALTER TABLE ... ADD
// COLUMN` statement — GenerateDDL's `CREATE TABLE IF NOT EXISTS` doesn't
// add columns to a table that already exists, so evolving a dataset
// (Store.AddColumns) needs this instead, applied only to the columns that
// are actually new.
func GenerateAlterDDL(datasetName string, newColumns []Column) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ALTER TABLE %s\n", quoteIdent(datasetName))
	for i, col := range newColumns {
		colType := col.Type
		if col.Nullable {
			colType = fmt.Sprintf("Nullable(%s)", colType)
		}
		sep := ","
		if i == len(newColumns)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "    ADD COLUMN %s %s%s\n", quoteIdent(col.Name), colType, sep)
	}
	return strings.TrimRight(b.String(), "\n")
}

// quoteIdent backtick-quotes a ClickHouse identifier. Datasets/columns
// come from InferColumns (JSON field names) or dataset-creation requests,
// neither validated against a strict charset, so quoting is the cheap way
// to avoid a column literally named `order` (a SQL keyword) breaking the
// generated DDL.
func quoteIdent(name string) string {
	escaped := strings.ReplaceAll(name, "`", "``")
	return "`" + escaped + "`"
}
