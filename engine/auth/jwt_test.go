package auth

import (
	"testing"
	"time"

	"herminas/kernel/permissions"
)

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	m := NewJWTManager([]byte("test-secret"), time.Hour)

	token, err := m.Issue("alice", permissions.RoleEngineer)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	claims, err := m.Verify(token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if claims.Username != "alice" || claims.Role != permissions.RoleEngineer {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	m := NewJWTManager([]byte("test-secret"), time.Millisecond)
	token, err := m.Issue("alice", permissions.RoleViewer)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if _, err := m.Verify(token); err == nil {
		t.Fatal("expected an error for an expired token")
	}
}

func TestVerifyRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	issuer := NewJWTManager([]byte("secret-a"), time.Hour)
	verifier := NewJWTManager([]byte("secret-b"), time.Hour)

	token, err := issuer.Issue("alice", permissions.RoleViewer)
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	if _, err := verifier.Verify(token); err == nil {
		t.Fatal("expected verification to fail with a mismatched secret")
	}
}

func TestVerifyRejectsGarbageToken(t *testing.T) {
	m := NewJWTManager([]byte("test-secret"), time.Hour)
	if _, err := m.Verify("not-a-jwt"); err == nil {
		t.Fatal("expected an error for a garbage token")
	}
}
