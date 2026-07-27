package domain

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAPIKeySecret_ShapeAndUniqueness(t *testing.T) {
	seen := make(map[string]bool, 128)
	for i := 0; i < 128; i++ {
		secret, prefix, err := GenerateAPIKeySecret()
		if err != nil {
			t.Fatalf("generating: %v", err)
		}
		if !strings.HasPrefix(secret, APIKeyPrefix) {
			t.Fatalf("secret must carry the %q prefix: %q", APIKeyPrefix, secret)
		}
		if !strings.HasPrefix(secret, prefix) {
			t.Fatalf("lookup prefix %q is not a prefix of the secret", prefix)
		}
		// base64url without padding: safe in a header and in a URL.
		if strings.ContainsAny(secret, "=+/") {
			t.Errorf("secret must be URL/header safe, got %q", secret)
		}
		if seen[secret] {
			t.Fatal("generated a duplicate secret — entropy is broken")
		}
		seen[secret] = true
	}
}

// The stored prefix must not be enough to reconstruct the secret.
func TestAPIKeyLookupPrefix_IsShortEnoughToBeUseless(t *testing.T) {
	secret, prefix, err := GenerateAPIKeySecret()
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	if len(prefix) >= len(secret) {
		t.Fatal("the lookup prefix must be strictly shorter than the secret")
	}
	if len(prefix) != apiKeyLookupPrefixLen {
		t.Errorf("prefix length = %d, want %d", len(prefix), apiKeyLookupPrefixLen)
	}
}

func TestHashAPIKeySecret_StableAndDistinct(t *testing.T) {
	a := HashAPIKeySecret("bs_one")
	if a != HashAPIKeySecret("bs_one") {
		t.Error("hashing must be deterministic")
	}
	if a == HashAPIKeySecret("bs_two") {
		t.Error("different secrets must not collide")
	}
	if len(a) != 64 {
		t.Errorf("expected hex sha-256 (64 chars), got %d", len(a))
	}
	if strings.Contains(a, "bs_one") {
		t.Error("the hash must not contain the secret")
	}
}

func TestAPIKeySecretMatches(t *testing.T) {
	secret := "bs_correct-horse-battery-staple"
	hash := HashAPIKeySecret(secret)

	if !APIKeySecretMatches(secret, hash) {
		t.Error("the right secret must match its hash")
	}
	if APIKeySecretMatches("bs_wrong", hash) {
		t.Error("a wrong secret must not match")
	}
	// A near-miss is the case a timing-unsafe comparison leaks on.
	if APIKeySecretMatches(secret+"x", hash) {
		t.Error("a secret with a trailing character must not match")
	}
	if APIKeySecretMatches("", hash) {
		t.Error("an empty secret must not match")
	}
}

func TestLooksLikeAPIKey(t *testing.T) {
	for _, tc := range []struct {
		credential string
		want       bool
	}{
		{"bs_abc123", true},
		{"eyJhbGciOiJIUzI1NiJ9.payload.sig", false}, // a JWT
		{"", false},
		{"BS_upper", false}, // prefix is case-sensitive on purpose
	} {
		if got := LooksLikeAPIKey(tc.credential); got != tc.want {
			t.Errorf("LooksLikeAPIKey(%q) = %v, want %v", tc.credential, got, tc.want)
		}
	}
}

func TestAPIKey_IsUsable(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	for _, tc := range []struct {
		name string
		key  *APIKey
		want bool
	}{
		{"live", &APIKey{}, true},
		{"revoked", &APIKey{RevokedAt: &past}, false},
		{"expired", &APIKey{ExpiresAt: &past}, false},
		{"not yet expired", &APIKey{ExpiresAt: &future}, true},
		{"expiring exactly now", &APIKey{ExpiresAt: &now}, false},
		{"revoked wins over valid expiry", &APIKey{RevokedAt: &past, ExpiresAt: &future}, false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.key.IsUsable(now); got != tc.want {
				t.Errorf("IsUsable() = %v, want %v", got, tc.want)
			}
		})
	}
}
