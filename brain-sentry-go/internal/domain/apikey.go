package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// APIKeyPrefix marks a secret as a brainsentry service key. Authentication
// dispatches on it: a credential starting with this is looked up as an API
// key, anything else falls through to JWT validation. Keeping the two apart
// by shape means adding key auth cannot change how an existing token is
// interpreted.
const APIKeyPrefix = "bs_"

// apiKeySecretBytes is the entropy of the random part. 32 bytes = 256 bits,
// which is what makes storing a plain SHA-256 (rather than a slow KDF) the
// right call: there is no dictionary to defend against.
const apiKeySecretBytes = 32

// apiKeyLookupPrefixLen is how much of the secret is stored in clear to find
// the candidate row. Long enough to be selective, short enough to be useless
// on its own — with 256 bits of entropy behind it, knowing 12 characters
// does not measurably help an attacker.
const apiKeyLookupPrefixLen = 12

// APIKey is a long-lived credential scoped to exactly ONE tenant.
//
// The scoping is the point. A user JWT carries a tenant claim that an ADMIN
// may override via X-Tenant-ID; a service key may not, ever (see
// middleware.TenantExtractor). That is the difference between "a bug writes
// to the wrong customer" and "a bug cannot reach the wrong customer".
type APIKey struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenantId"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"keyPrefix"`
	// KeyHash never leaves the repository layer; json:"-" so it cannot be
	// serialised into a response by accident.
	KeyHash    string     `json:"-"`
	CreatedAt  time.Time  `json:"createdAt"`
	CreatedBy  string     `json:"createdBy,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

// IsUsable reports whether the key may authenticate a request right now.
// Revocation and expiry are checked here, in one place, so no call site can
// forget one of them.
func (k *APIKey) IsUsable(now time.Time) bool {
	if k == nil {
		return false
	}
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && !now.Before(*k.ExpiresAt) {
		return false
	}
	return true
}

// GenerateAPIKeySecret returns a new secret and its lookup prefix. The secret
// is the ONLY time the caller sees it — everything downstream stores the
// hash.
//
// Encoding is base64url without padding: URL-safe, header-safe, and no '='
// to trip up naive parsers.
func GenerateAPIKeySecret() (secret, prefix string, err error) {
	buf := make([]byte, apiKeySecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generating api key: %w", err)
	}
	secret = APIKeyPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return secret, APIKeyLookupPrefix(secret), nil
}

// APIKeyLookupPrefix returns the clear-text handle stored alongside the hash.
// Callers use it to find the candidate row; the authorisation decision is
// always the constant-time hash comparison, never this.
func APIKeyLookupPrefix(secret string) string {
	if len(secret) <= apiKeyLookupPrefixLen {
		return secret
	}
	return secret[:apiKeyLookupPrefixLen]
}

// HashAPIKeySecret returns the hex SHA-256 of the secret.
//
// Deliberately not bcrypt/argon2: those exist to slow down guessing of
// low-entropy human passwords. This input is 256 random bits, so a slow KDF
// buys nothing measurable and costs ~100ms on every authenticated request —
// on a recall path whose whole budget is 800ms (RFC-014 §6.3).
func HashAPIKeySecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// LooksLikeAPIKey reports whether a credential should be treated as a service
// key rather than a JWT.
func LooksLikeAPIKey(credential string) bool {
	return strings.HasPrefix(credential, APIKeyPrefix)
}

// APIKeySecretMatches compares a presented secret against a stored hash in
// constant time.
//
// subtle.ConstantTimeCompare matters even though the hashes are public-ish:
// a byte-by-byte comparison leaks, through timing, how much of a guess is
// correct — which turns an infeasible search into a feasible one.
func APIKeySecretMatches(presentedSecret, storedHash string) bool {
	computed := HashAPIKeySecret(presentedSecret)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(storedHash)) == 1
}
