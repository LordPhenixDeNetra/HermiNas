package schemamgr

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: no CGO, keeps the control-plane binary statically linkable

	"herminas/kernel/errors"
)

// Store persists dataset definitions and their full version history in
// SQLite (cahier des charges §9.2: herminas.db). Pure-Go driver
// (modernc.org/sqlite) so `CGO_ENABLED=0` static builds still work.
type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "open sqlite", err)
	}

	const schema = `
CREATE TABLE IF NOT EXISTS datasets (
    name       TEXT PRIMARY KEY,
    definition TEXT NOT NULL,
    version    INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS dataset_versions (
    name       TEXT NOT NULL,
    version    INTEGER NOT NULL,
    definition TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (name, version)
);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, errors.Wrap(errors.CodeInternal, "create schemamgr tables", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Create registers a brand-new dataset at version 1. Fails if one with the
// same name already exists — use AddColumns to evolve it instead.
func (s *Store) Create(d Dataset) (Dataset, error) {
	if err := d.Validate(); err != nil {
		return Dataset{}, err
	}

	if _, err := s.Get(d.Name); err == nil {
		return Dataset{}, errors.New(errors.CodeAlreadyExists, fmt.Sprintf("dataset %q already exists", d.Name))
	}

	now := time.Now().UTC()
	d.Version = 1
	d.CreatedAt = now
	d.UpdatedAt = now

	if err := s.persist(d); err != nil {
		return Dataset{}, err
	}
	return d, nil
}

// AddColumns evolves an existing dataset by appending newColumns,
// rejecting the change if it would remove or retype anything
// (CheckBackwardCompatible). Bumps Version and appends a new
// dataset_versions row — the old version stays queryable via Versions.
func (s *Store) AddColumns(name string, newColumns []Column) (Dataset, error) {
	current, err := s.Get(name)
	if err != nil {
		return Dataset{}, err
	}

	next := current
	next.Columns = append(append([]Column{}, current.Columns...), newColumns...)
	next.Version = current.Version + 1
	next.UpdatedAt = time.Now().UTC()

	if err := next.Validate(); err != nil {
		return Dataset{}, err
	}
	if err := CheckBackwardCompatible(current, next); err != nil {
		return Dataset{}, err
	}

	if err := s.persist(next); err != nil {
		return Dataset{}, err
	}
	return next, nil
}

// EnsureDataset returns the existing dataset, or — if none exists yet —
// infers one from sample and creates it (M1.3: "Auto-création de dataset
// à la première ingestion, schéma inféré"). The bool result reports
// whether a dataset was just created.
func (s *Store) EnsureDataset(name string, sample map[string]any) (Dataset, bool, error) {
	existing, err := s.Get(name)
	if err == nil {
		return existing, false, nil
	}
	if !errors.IsNotFound(err) {
		return Dataset{}, false, err
	}

	columns := InferColumns(sample)
	d := Dataset{Name: name, Columns: columns}

	// received_at_unix_ms is the receiver's own enrichment field (M1.2) —
	// when present, it's a reasonable default sort key. No
	// PartitionByColumn/TTL: JSON has no native date type to partition on
	// safely (see infer.go), so auto-created datasets stay unpartitioned
	// until a human sets one explicitly via AddColumns/DDL review.
	for _, c := range columns {
		if c.Name == "received_at_unix_ms" {
			d.OrderBy = []string{c.Name}
			break
		}
	}

	created, err := s.Create(d)
	if err != nil {
		return Dataset{}, false, err
	}
	return created, true, nil
}

// Get returns the latest version of a dataset. Returns a NotFound
// *errors.Error if it doesn't exist.
func (s *Store) Get(name string) (Dataset, error) {
	var definition string
	err := s.db.QueryRow(`SELECT definition FROM datasets WHERE name = ?`, name).Scan(&definition)
	if err == sql.ErrNoRows {
		return Dataset{}, errors.New(errors.CodeNotFound, fmt.Sprintf("dataset %q not found", name))
	}
	if err != nil {
		return Dataset{}, errors.Wrap(errors.CodeInternal, "query dataset", err)
	}

	var d Dataset
	if err := json.Unmarshal([]byte(definition), &d); err != nil {
		return Dataset{}, errors.Wrap(errors.CodeInternal, "decode dataset definition", err)
	}
	return d, nil
}

// List returns every dataset's latest version, ordered by name.
func (s *Store) List() ([]Dataset, error) {
	rows, err := s.db.Query(`SELECT definition FROM datasets ORDER BY name`)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "list datasets", err)
	}
	defer rows.Close()

	var out []Dataset
	for rows.Next() {
		var definition string
		if err := rows.Scan(&definition); err != nil {
			return nil, errors.Wrap(errors.CodeInternal, "scan dataset row", err)
		}
		var d Dataset
		if err := json.Unmarshal([]byte(definition), &d); err != nil {
			return nil, errors.Wrap(errors.CodeInternal, "decode dataset definition", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Versions returns every persisted version of name, oldest first — the
// registry history behind "compatibilité ascendante".
func (s *Store) Versions(name string) ([]Dataset, error) {
	rows, err := s.db.Query(
		`SELECT definition FROM dataset_versions WHERE name = ? ORDER BY version ASC`, name,
	)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "list dataset versions", err)
	}
	defer rows.Close()

	var out []Dataset
	for rows.Next() {
		var definition string
		if err := rows.Scan(&definition); err != nil {
			return nil, errors.Wrap(errors.CodeInternal, "scan dataset version row", err)
		}
		var d Dataset
		if err := json.Unmarshal([]byte(definition), &d); err != nil {
			return nil, errors.Wrap(errors.CodeInternal, "decode dataset version", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) persist(d Dataset) error {
	encoded, err := json.Marshal(d)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "encode dataset definition", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "begin transaction", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	_, err = tx.Exec(
		`INSERT INTO datasets (name, definition, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET definition = excluded.definition, version = excluded.version, updated_at = excluded.updated_at`,
		d.Name, string(encoded), d.Version, d.CreatedAt.Format(time.RFC3339Nano), d.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "upsert dataset", err)
	}

	_, err = tx.Exec(
		`INSERT INTO dataset_versions (name, version, definition, created_at) VALUES (?, ?, ?, ?)`,
		d.Name, d.Version, string(encoded), d.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "insert dataset version", err)
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(errors.CodeInternal, "commit transaction", err)
	}
	return nil
}
