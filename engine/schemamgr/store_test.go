package schemamgr

import (
	"path/filepath"
	"testing"

	"herminas/kernel/errors"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "herminas.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCreateAndGet(t *testing.T) {
	store := openTestStore(t)

	d := Dataset{Name: "logs", Columns: []Column{{Name: "message", Type: "String"}}}
	created, err := store.Create(d)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("expected version 1, got %d", created.Version)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	got, err := store.Get("logs")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "logs" || len(got.Columns) != 1 {
		t.Fatalf("unexpected dataset: %+v", got)
	}
}

func TestCreateRejectsDuplicateName(t *testing.T) {
	store := openTestStore(t)
	d := Dataset{Name: "logs", Columns: []Column{{Name: "a", Type: "String"}}}

	if _, err := store.Create(d); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	_, err := store.Create(d)
	if !errors.IsAlreadyExists(err) {
		t.Fatalf("expected already_exists, got %v", err)
	}
}

func TestGetUnknownDatasetReturnsNotFound(t *testing.T) {
	store := openTestStore(t)
	_, err := store.Get("nope")
	if !errors.IsNotFound(err) {
		t.Fatalf("expected not_found, got %v", err)
	}
}

func TestListReturnsAllDatasetsSortedByName(t *testing.T) {
	store := openTestStore(t)
	for _, name := range []string{"zebra", "alpha"} {
		if _, err := store.Create(Dataset{Name: name, Columns: []Column{{Name: "a", Type: "String"}}}); err != nil {
			t.Fatalf("Create(%s) failed: %v", name, err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 || list[0].Name != "alpha" || list[1].Name != "zebra" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestAddColumnsBumpsVersionAndKeepsHistory(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Create(Dataset{Name: "logs", Columns: []Column{{Name: "message", Type: "String"}}}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	evolved, err := store.AddColumns("logs", []Column{{Name: "level", Type: "String"}})
	if err != nil {
		t.Fatalf("AddColumns failed: %v", err)
	}
	if evolved.Version != 2 || len(evolved.Columns) != 2 {
		t.Fatalf("unexpected evolved dataset: %+v", evolved)
	}

	versions, err := store.Versions("logs")
	if err != nil {
		t.Fatalf("Versions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if len(versions[0].Columns) != 1 || len(versions[1].Columns) != 2 {
		t.Fatalf("expected version history to reflect column growth: %+v", versions)
	}
}

func TestAddColumnsRejectsIncompatibleChange(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.Create(Dataset{Name: "logs", Columns: []Column{{Name: "message", Type: "String"}}}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Same name, different type: AddColumns appends rather than replacing,
	// so this becomes a duplicate-name Validate error — either way, the
	// evolution must be rejected rather than silently corrupting the schema.
	_, err := store.AddColumns("logs", []Column{{Name: "message", Type: "Int64"}})
	if err == nil {
		t.Fatal("expected AddColumns to reject a conflicting column definition")
	}
}

func TestEnsureDatasetCreatesOnFirstSightingThenReuses(t *testing.T) {
	store := openTestStore(t)
	sample := map[string]any{"level": "info", "message": "hello"}

	d1, created1, err := store.EnsureDataset("logs", sample)
	if err != nil {
		t.Fatalf("EnsureDataset failed: %v", err)
	}
	if !created1 {
		t.Fatal("expected first EnsureDataset call to create the dataset")
	}

	d2, created2, err := store.EnsureDataset("logs", sample)
	if err != nil {
		t.Fatalf("second EnsureDataset failed: %v", err)
	}
	if created2 {
		t.Fatal("expected second EnsureDataset call to reuse the existing dataset")
	}
	if d1.Version != d2.Version {
		t.Fatalf("expected the same dataset version, got %d vs %d", d1.Version, d2.Version)
	}
}

func TestEnsureDatasetOrdersByReceivedAtWhenPresent(t *testing.T) {
	store := openTestStore(t)
	sample := map[string]any{"message": "hi", "received_at_unix_ms": float64(123)}

	d, _, err := store.EnsureDataset("logs", sample)
	if err != nil {
		t.Fatalf("EnsureDataset failed: %v", err)
	}
	if len(d.OrderBy) != 1 || d.OrderBy[0] != "received_at_unix_ms" {
		t.Fatalf("expected OrderBy=[received_at_unix_ms], got %v", d.OrderBy)
	}
}
