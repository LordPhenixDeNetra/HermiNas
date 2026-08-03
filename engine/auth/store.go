// Package auth owns HermiNas' users, API tokens and JWT sessions (M1.5).
// Passwords are bcrypt-hashed; API tokens are stored as a SHA-256 hash of
// the raw value, never the value itself — same "never store what you
// don't have to reveal again" discipline as kernel/settings.SecretString
// (M0.4), just for a database row instead of a config field.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"

	"herminas/kernel/errors"
	"herminas/kernel/permissions"
)

type User struct {
	Username     string           `json:"username"`
	PasswordHash string           `json:"-"` // never serialized back to a client
	Role         permissions.Role `json:"role"`
	CreatedAt    time.Time        `json:"created_at"`
}

type APIToken struct {
	ID        string           `json:"id"`
	Scopes    []string         `json:"scopes"` // dataset names, or ["*"] for every dataset
	Role      permissions.Role `json:"role"`
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
	Revoked   bool             `json:"revoked"`
}

// AllowsDataset reports whether this token's scope covers dataset.
func (t APIToken) AllowsDataset(dataset string) bool {
	for _, s := range t.Scopes {
		if s == "*" || s == dataset {
			return true
		}
	}
	return false
}

func (t APIToken) expired() bool {
	return t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt)
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "open auth store", err)
	}

	const schema = `
CREATE TABLE IF NOT EXISTS users (
    username      TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL,
    created_at    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS api_tokens (
    id          TEXT PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,
    scopes      TEXT NOT NULL,
    role        TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    expires_at  TEXT,
    revoked     INTEGER NOT NULL DEFAULT 0
);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, errors.Wrap(errors.CodeInternal, "create auth tables", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// CreateUser hashes password with bcrypt before it ever touches disk.
func (s *Store) CreateUser(username, password string, role permissions.Role) (User, error) {
	if !permissions.Valid(role) {
		return User{}, errors.New(errors.CodeInvalidArgument, "unknown role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, errors.Wrap(errors.CodeInternal, "hash password", err)
	}

	u := User{Username: username, PasswordHash: string(hash), Role: role, CreatedAt: time.Now().UTC()}
	_, err = s.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, string(u.Role), u.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return User{}, errors.Wrap(errors.CodeAlreadyExists, "create user", err)
	}
	return u, nil
}

// VerifyPassword returns the user if username exists and password matches
// its bcrypt hash. Same error either way (unknown user vs. wrong
// password) — distinguishing them would tell an attacker which usernames
// exist.
func (s *Store) VerifyPassword(username, password string) (User, error) {
	var u User
	var createdAt string
	var role string
	err := s.db.QueryRow(`SELECT username, password_hash, role, created_at FROM users WHERE username = ?`, username).
		Scan(&u.Username, &u.PasswordHash, &role, &createdAt)
	if err != nil {
		return User{}, errors.New(errors.CodeUnauthorized, "invalid username or password")
	}
	u.Role = permissions.Role(role)
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)

	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return User{}, errors.New(errors.CodeUnauthorized, "invalid username or password")
	}
	return u, nil
}

// CreateAPIToken generates a random token, persists only its hash, and
// returns the raw value once — the caller (an admin, via the API) must
// save it now, since it can never be retrieved again.
func (s *Store) CreateAPIToken(scopes []string, role permissions.Role, ttl time.Duration) (rawToken string, token APIToken, err error) {
	if !permissions.Valid(role) {
		return "", APIToken{}, errors.New(errors.CodeInvalidArgument, "unknown role")
	}
	if len(scopes) == 0 {
		return "", APIToken{}, errors.New(errors.CodeInvalidArgument, "at least one scope is required (use \"*\" for all datasets)")
	}

	raw, err := randomToken()
	if err != nil {
		return "", APIToken{}, errors.Wrap(errors.CodeInternal, "generate token", err)
	}
	id, err := randomToken()
	if err != nil {
		return "", APIToken{}, errors.Wrap(errors.CodeInternal, "generate token id", err)
	}

	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return "", APIToken{}, errors.Wrap(errors.CodeInternal, "encode scopes", err)
	}

	t := APIToken{ID: id, Scopes: scopes, Role: role, CreatedAt: time.Now().UTC()}
	if ttl > 0 {
		exp := t.CreatedAt.Add(ttl)
		t.ExpiresAt = &exp
	}

	var expiresAt any
	if t.ExpiresAt != nil {
		expiresAt = t.ExpiresAt.Format(time.RFC3339Nano)
	}

	_, err = s.db.Exec(
		`INSERT INTO api_tokens (id, token_hash, scopes, role, created_at, expires_at, revoked) VALUES (?, ?, ?, ?, ?, ?, 0)`,
		t.ID, hashToken(raw), string(scopesJSON), string(t.Role), t.CreatedAt.Format(time.RFC3339Nano), expiresAt,
	)
	if err != nil {
		return "", APIToken{}, errors.Wrap(errors.CodeInternal, "store api token", err)
	}
	return raw, t, nil
}

// VerifyAPIToken looks up rawToken by its hash and rejects it if revoked
// or expired.
func (s *Store) VerifyAPIToken(rawToken string) (APIToken, error) {
	var t APIToken
	var scopesJSON, role, createdAt string
	var expiresAt sql.NullString
	var revoked bool

	err := s.db.QueryRow(
		`SELECT id, scopes, role, created_at, expires_at, revoked FROM api_tokens WHERE token_hash = ?`,
		hashToken(rawToken),
	).Scan(&t.ID, &scopesJSON, &role, &createdAt, &expiresAt, &revoked)
	if err != nil {
		return APIToken{}, errors.New(errors.CodeUnauthorized, "invalid API token")
	}

	if err := json.Unmarshal([]byte(scopesJSON), &t.Scopes); err != nil {
		return APIToken{}, errors.Wrap(errors.CodeInternal, "decode token scopes", err)
	}
	t.Role = permissions.Role(role)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	t.Revoked = revoked
	if expiresAt.Valid {
		parsed, _ := time.Parse(time.RFC3339Nano, expiresAt.String)
		t.ExpiresAt = &parsed
	}

	if t.Revoked {
		return APIToken{}, errors.New(errors.CodeUnauthorized, "API token has been revoked")
	}
	if t.expired() {
		return APIToken{}, errors.New(errors.CodeUnauthorized, "API token has expired")
	}
	return t, nil
}

func (s *Store) RevokeAPIToken(id string) error {
	res, err := s.db.Exec(`UPDATE api_tokens SET revoked = 1 WHERE id = ?`, id)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "revoke api token", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New(errors.CodeNotFound, "unknown token id")
	}
	return nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
