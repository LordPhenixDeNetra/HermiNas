package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"herminas/kernel/errors"
	"herminas/kernel/permissions"
)

// Claims is a UI session's JWT payload: who, and at what role, as of
// issuance. The role is baked into the token rather than looked up fresh
// on every request — a role change takes effect on the user's next login,
// which is an accepted tradeoff for M1.5's short-lived sessions (real
// revocation-on-change lands with fuller RBAC in M7.1).
type Claims struct {
	jwt.RegisteredClaims
	Username string           `json:"username"`
	Role     permissions.Role `json:"role"`
}

// JWTManager issues and verifies short-lived UI session tokens (cahier des
// charges §7.1: "Sessions UI | JWT courte durée + refresh").
type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret []byte, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: secret, ttl: ttl}
}

func (m *JWTManager) Issue(username string, role permissions.Role) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			Subject:   username,
		},
		Username: username,
		Role:     role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", errors.Wrap(errors.CodeInternal, "sign JWT", err)
	}
	return signed, nil
}

func (m *JWTManager) Verify(tokenString string) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return Claims{}, errors.New(errors.CodeUnauthorized, "invalid or expired session token")
	}
	return claims, nil
}
