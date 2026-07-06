package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Claims is the JWT payload: standard registered claims plus the role and
// branch needed to authorize requests without a database round-trip.
type Claims struct {
	jwt.RegisteredClaims
	Role     store.UserRole `json:"role"`
	BranchID *uuid.UUID     `json:"branch_id,omitempty"`
}

// TokenManager issues and verifies HS256 access tokens. Construct one with
// NewTokenManager and share it — it is safe for concurrent use.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// NewTokenManager returns a TokenManager signing with secret and issuing tokens
// valid for ttl.
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl, issuer: "saydalah-api"}
}

// Issue signs a new access token for the identity and returns it with its
// expiry time.
func (m *TokenManager) Issue(id Identity, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(m.ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   id.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Role:     id.Role,
		BranchID: id.BranchID,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse verifies a token string and returns the Identity it represents. It
// rejects tokens signed with an unexpected algorithm or lacking an expiry.
func (m *TokenManager) Parse(tokenString string) (Identity, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(m.issuer),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: invalid token: %w", err)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: invalid subject: %w", err)
	}
	return Identity{UserID: userID, Role: claims.Role, BranchID: claims.BranchID}, nil
}
