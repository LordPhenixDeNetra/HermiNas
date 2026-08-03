package auth

import (
	"path/filepath"
	"testing"
	"time"

	"herminas/kernel/errors"
	"herminas/kernel/permissions"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCreateUserAndVerifyPassword(t *testing.T) {
	store := openTestStore(t)

	if _, err := store.CreateUser("alice", "correct-horse", permissions.RoleAnalyst); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	u, err := store.VerifyPassword("alice", "correct-horse")
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if u.Username != "alice" || u.Role != permissions.RoleAnalyst {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	store := openTestStore(t)
	store.CreateUser("alice", "correct-horse", permissions.RoleAnalyst)

	if _, err := store.VerifyPassword("alice", "wrong"); !errors.Is(err, errors.CodeUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestVerifyPasswordRejectsUnknownUser(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.VerifyPassword("nobody", "whatever"); !errors.Is(err, errors.CodeUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestPasswordHashIsNeverStoredInPlaintext(t *testing.T) {
	store := openTestStore(t)
	u, _ := store.CreateUser("alice", "correct-horse", permissions.RoleAnalyst)
	if u.PasswordHash == "correct-horse" {
		t.Fatal("password hash must not equal the plaintext password")
	}
	if len(u.PasswordHash) == 0 {
		t.Fatal("expected a non-empty bcrypt hash")
	}
}

func TestCreateAPITokenAndVerify(t *testing.T) {
	store := openTestStore(t)

	raw, created, err := store.CreateAPIToken([]string{"logs", "metrics"}, permissions.RoleEngineer, 0)
	if err != nil {
		t.Fatalf("CreateAPIToken failed: %v", err)
	}
	if raw == "" {
		t.Fatal("expected a non-empty raw token")
	}

	verified, err := store.VerifyAPIToken(raw)
	if err != nil {
		t.Fatalf("VerifyAPIToken failed: %v", err)
	}
	if verified.ID != created.ID || verified.Role != permissions.RoleEngineer {
		t.Fatalf("unexpected token: %+v", verified)
	}
	if !verified.AllowsDataset("logs") || verified.AllowsDataset("other") {
		t.Fatalf("unexpected scope check result: %+v", verified)
	}
}

func TestAPITokenWildcardScopeAllowsAnyDataset(t *testing.T) {
	store := openTestStore(t)
	raw, _, err := store.CreateAPIToken([]string{"*"}, permissions.RoleAdmin, 0)
	if err != nil {
		t.Fatalf("CreateAPIToken failed: %v", err)
	}
	verified, err := store.VerifyAPIToken(raw)
	if err != nil {
		t.Fatalf("VerifyAPIToken failed: %v", err)
	}
	if !verified.AllowsDataset("anything") {
		t.Fatal("wildcard scope should allow any dataset")
	}
}

func TestVerifyAPITokenRejectsUnknownToken(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.VerifyAPIToken("not-a-real-token"); !errors.Is(err, errors.CodeUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestRevokedTokenIsRejected(t *testing.T) {
	store := openTestStore(t)
	raw, created, _ := store.CreateAPIToken([]string{"*"}, permissions.RoleViewer, 0)

	if err := store.RevokeAPIToken(created.ID); err != nil {
		t.Fatalf("RevokeAPIToken failed: %v", err)
	}
	if _, err := store.VerifyAPIToken(raw); !errors.Is(err, errors.CodeUnauthorized) {
		t.Fatalf("expected unauthorized for a revoked token, got %v", err)
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	store := openTestStore(t)
	raw, _, err := store.CreateAPIToken([]string{"*"}, permissions.RoleViewer, time.Millisecond)
	if err != nil {
		t.Fatalf("CreateAPIToken failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if _, err := store.VerifyAPIToken(raw); !errors.Is(err, errors.CodeUnauthorized) {
		t.Fatalf("expected unauthorized for an expired token, got %v", err)
	}
}

func TestCreateAPITokenRequiresAtLeastOneScope(t *testing.T) {
	store := openTestStore(t)
	if _, _, err := store.CreateAPIToken(nil, permissions.RoleViewer, 0); !errors.IsInvalidArgument(err) {
		t.Fatalf("expected invalid_argument, got %v", err)
	}
}
