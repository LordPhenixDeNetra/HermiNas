package schemamgr

import (
	"testing"

	"herminas/kernel/errors"
)

func TestValidateRejectsEmptyName(t *testing.T) {
	d := Dataset{Columns: []Column{{Name: "a", Type: "String"}}}
	if err := d.Validate(); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument, got %v", err)
	}
}

func TestValidateRejectsNoColumns(t *testing.T) {
	d := Dataset{Name: "logs"}
	if err := d.Validate(); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument, got %v", err)
	}
}

func TestValidateRejectsDuplicateColumn(t *testing.T) {
	d := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}, {Name: "a", Type: "Int64"}}}
	if err := d.Validate(); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument, got %v", err)
	}
}

func TestValidateRejectsUnknownOrderByColumn(t *testing.T) {
	d := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}, OrderBy: []string{"nope"}}
	if err := d.Validate(); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument, got %v", err)
	}
}

func TestValidateRejectsTTLWithoutPartitionColumn(t *testing.T) {
	d := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}, TTLDays: 30}
	if err := d.Validate(); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument, got %v", err)
	}
}

func TestValidateAcceptsWellFormedDataset(t *testing.T) {
	d := Dataset{
		Name:              "logs",
		Columns:           []Column{{Name: "ts", Type: "DateTime"}, {Name: "msg", Type: "String"}},
		OrderBy:           []string{"ts"},
		PartitionByColumn: "ts",
		TTLDays:           30,
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected valid dataset, got %v", err)
	}
}

func TestCheckBackwardCompatibleAllowsAddingColumns(t *testing.T) {
	previous := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}}
	next := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}, {Name: "b", Type: "Int64"}}}
	if err := CheckBackwardCompatible(previous, next); err != nil {
		t.Fatalf("adding a column should be compatible: %v", err)
	}
}

func TestCheckBackwardCompatibleRejectsRemovingColumn(t *testing.T) {
	previous := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}, {Name: "b", Type: "Int64"}}}
	next := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}}
	if err := CheckBackwardCompatible(previous, next); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument for removed column, got %v", err)
	}
}

func TestCheckBackwardCompatibleRejectsRetypingColumn(t *testing.T) {
	previous := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}}
	next := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "Int64"}}}
	if err := CheckBackwardCompatible(previous, next); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument for retyped column, got %v", err)
	}
}

func TestCheckBackwardCompatibleRejectsMakingColumnNonNullable(t *testing.T) {
	previous := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String", Nullable: true}}}
	next := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String", Nullable: false}}}
	if err := CheckBackwardCompatible(previous, next); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument for narrowed nullability, got %v", err)
	}
}
