// Package schemamgr owns dataset definitions (M1.3): name, columns, TTL,
// generates the ClickHouse MergeTree DDL for them, persists a versioned
// registry in SQLite, and can auto-create a dataset from a sample record
// (schema inference) the first time an unknown dataset is ingested.
package schemamgr

import (
	"fmt"
	"time"

	"herminas/kernel/errors"
)

// Column is one field of a dataset. Type is a ClickHouse type name
// (String, Int64, Float64, Bool, DateTime64(3)) — see infer.go for how
// it's chosen from a sample JSON value.
type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// Dataset is one table's definition: what schemamgr persists, versions,
// and turns into DDL (ddl.go).
type Dataset struct {
	Name string   `json:"name"`
	Columns []Column `json:"columns"`
	// OrderBy is the MergeTree ORDER BY key. Empty means ORDER BY tuple()
	// (no sort key) — valid ClickHouse, just no sort-derived query speedup.
	OrderBy []string `json:"order_by"`
	// PartitionByColumn, if set, must name a DateTime-ish column; DDL
	// partitions by toYYYYMM() of it. Empty means no partitioning.
	PartitionByColumn string `json:"partition_by_column"`
	// TTLDays, if > 0, adds `TTL {PartitionByColumn} + INTERVAL n DAY`.
	// Requires PartitionByColumn (or another DateTime column) to exist.
	TTLDays int `json:"ttl_days"`

	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (d Dataset) column(name string) (Column, bool) {
	for _, c := range d.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}

// Validate checks a dataset definition is internally consistent before
// it's ever persisted or turned into DDL.
func (d Dataset) Validate() error {
	if d.Name == "" {
		return errors.New(errors.CodeInvalidArgument, "dataset name is required")
	}
	if len(d.Columns) == 0 {
		return errors.New(errors.CodeInvalidArgument, "dataset must have at least one column")
	}
	seen := make(map[string]bool, len(d.Columns))
	for _, c := range d.Columns {
		if c.Name == "" || c.Type == "" {
			return errors.New(errors.CodeInvalidArgument, "column name and type are required")
		}
		if seen[c.Name] {
			return errors.New(errors.CodeInvalidArgument, fmt.Sprintf("duplicate column %q", c.Name))
		}
		seen[c.Name] = true
	}
	for _, ob := range d.OrderBy {
		if _, ok := d.column(ob); !ok {
			return errors.New(errors.CodeInvalidArgument, fmt.Sprintf("order_by column %q not defined", ob))
		}
	}
	if d.PartitionByColumn != "" {
		if _, ok := d.column(d.PartitionByColumn); !ok {
			return errors.New(errors.CodeInvalidArgument, fmt.Sprintf("partition_by_column %q not defined", d.PartitionByColumn))
		}
	}
	if d.TTLDays > 0 && d.PartitionByColumn == "" {
		return errors.New(errors.CodeInvalidArgument, "ttl_days requires partition_by_column (TTL needs a date/time column)")
	}
	return nil
}

// CheckBackwardCompatible enforces "compatibilité ascendante" (M1.3):
// evolving a dataset may only *add* columns. Removing a column or changing
// an existing one's type would break every query and every already-shipped
// row written under the old definition, so both are rejected.
func CheckBackwardCompatible(previous, next Dataset) error {
	for _, oldCol := range previous.Columns {
		newCol, ok := next.column(oldCol.Name)
		if !ok {
			return errors.New(errors.CodeInvalidArgument, fmt.Sprintf("cannot remove column %q", oldCol.Name))
		}
		if newCol.Type != oldCol.Type {
			return errors.New(errors.CodeInvalidArgument, fmt.Sprintf("cannot change column %q type from %s to %s", oldCol.Name, oldCol.Type, newCol.Type))
		}
		if newCol.Nullable != oldCol.Nullable && oldCol.Nullable {
			return errors.New(errors.CodeInvalidArgument, fmt.Sprintf("cannot make column %q non-nullable", oldCol.Name))
		}
	}
	return nil
}
