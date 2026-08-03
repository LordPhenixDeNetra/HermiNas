package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"herminas/kernel/permissions"
)

// Identity is what Authenticate resolves a request's credentials to,
// whichever of the two schemes matched (cahier des charges §7.1: JWT for
// UI sessions, opaque tokens for the public API).
type Identity struct {
	Subject  string // username (JWT) or token ID (API token)
	Role     permissions.Role
	Scopes   []string // dataset scopes; nil for JWT sessions (role-only, not dataset-scoped yet)
	ViaToken bool
}

// AllowsDataset reports whether this identity may touch dataset. JWT
// sessions aren't dataset-scoped (M1.5 scope is role-only for them; full
// per-dataset UI permissions is a later RBAC refinement, M7.1) — only API
// tokens carry an explicit scope list.
func (i Identity) AllowsDataset(dataset string) bool {
	if !i.ViaToken {
		return true
	}
	for _, s := range i.Scopes {
		if s == "*" || s == dataset {
			return true
		}
	}
	return false
}

type contextKey int

const identityKey contextKey = iota

func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFromContext returns the identity Authenticate resolved for this
// request. Only meaningful inside a handler wrapped by Authenticate.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// Authenticate accepts either scheme behind a single `Authorization:
// Bearer <value>` header: a JWT (checked first) or an opaque API token.
// Trying JWT verification is cheap (local HMAC check, no DB hit) so
// ordering it first doesn't cost API-token requests anything beyond one
// failed local verification.
func Authenticate(jwtManager *JWTManager, store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if header == "" || !ok || token == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}

			if claims, err := jwtManager.Verify(token); err == nil {
				ctx := withIdentity(r.Context(), Identity{Subject: claims.Username, Role: claims.Role})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if apiToken, err := store.VerifyAPIToken(token); err == nil {
				ctx := withIdentity(r.Context(), Identity{
					Subject:  apiToken.ID,
					Role:     apiToken.Role,
					Scopes:   apiToken.Scopes,
					ViaToken: true,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			writeAuthError(w, http.StatusUnauthorized, "invalid or expired credentials")
		})
	}
}

// RequireRole rejects the request unless the authenticated identity's role
// is at least min (kernel/permissions' 4-role hierarchy).
func RequireRole(min permissions.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !permissions.AtLeast(id.Role, min) {
				writeAuthError(w, http.StatusForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireDatasetScope rejects the request unless the identity's token
// scope covers the dataset datasetName extracts from it (typically
// r.PathValue("dataset")). JWT sessions always pass (see
// Identity.AllowsDataset).
func RequireDatasetScope(datasetName func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !id.AllowsDataset(datasetName(r)) {
				writeAuthError(w, http.StatusForbidden, "token does not have access to this dataset")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
