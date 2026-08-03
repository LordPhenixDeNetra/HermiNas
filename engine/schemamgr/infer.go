package schemamgr

import (
	"sort"
)

// InferColumns derives a column list from one sample record — used to
// auto-create a dataset the first time an unknown one is ingested (M1.3:
// "Auto-création de dataset à la première ingestion, schéma inféré").
// Columns come out sorted by name so inference is deterministic: the same
// sample always produces the same DDL, which matters once dataset
// definitions are hashed/compared for compatibility checks.
func InferColumns(sample map[string]any) []Column {
	columns := make([]Column, 0, len(sample))
	for name, value := range sample {
		columns = append(columns, Column{
			Name:     name,
			Type:     inferType(value),
			Nullable: value == nil,
		})
	}
	sort.Slice(columns, func(i, j int) bool { return columns[i].Name < columns[j].Name })
	return columns
}

// inferType maps a decoded JSON value to a ClickHouse column type. Nested
// objects/arrays fall back to String (holding their JSON text) rather than
// a nested ClickHouse type — matches the flattening rust/agent's parse
// module already does on the ingestion side, so a record's shape is
// interpreted the same way at both ends of the pipeline.
func inferType(value any) string {
	switch v := value.(type) {
	case nil:
		return "String"
	case bool:
		return "Bool"
	case float64:
		if v == float64(int64(v)) {
			return "Int64"
		}
		return "Float64"
	case string:
		return "String"
	default:
		// map[string]any, []any, or anything else json.Unmarshal produced:
		// store the JSON text rather than modeling nested structure.
		return "String"
	}
}
