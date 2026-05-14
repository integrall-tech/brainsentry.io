package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/integraltech/brainsentry/internal/cache"
)

// AdminTrustHandler exposes operator-only endpoints whose blast radius is
// wide enough that they refuse anything below trust.Local. Wiring lives in
// cmd/server/main.go; routes must be wrapped in middleware.RequireLocalTrust.
//
// We deliberately do not put a UI button on these — they exist so the CLI
// (which elevates trust to Local) can drive them, and so a future fully
// authenticated localhost-only admin could too. From the network they are
// invisible by contract.
type AdminTrustHandler struct {
	Cache *cache.RedisCache
}

// NewAdminTrustHandler builds the handler. cache may be nil (Redis not
// configured); methods then return 503.
func NewAdminTrustHandler(c *cache.RedisCache) *AdminTrustHandler {
	return &AdminTrustHandler{Cache: c}
}

// WipeEmbeddingCache handles POST /v1/admin/wipe-embedding-cache. Removes
// every Redis key under the embedding prefix. Idempotent. Loud — every
// caller will pay for re-embedding on the next search until the cache
// repopulates.
func (h *AdminTrustHandler) WipeEmbeddingCache(w http.ResponseWriter, r *http.Request) {
	if h.Cache == nil {
		writeError(w, http.StatusServiceUnavailable, "redis cache not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	deleted, err := wipeKeysByPrefix(ctx, h.Cache, "embedding:")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"wiped":   deleted,
		"prefix":  "embedding:",
		"message": "embedding cache cleared; next searches will re-embed lazily",
	})
}

// wipeKeysByPrefix scans Redis for keys matching prefix* and deletes them
// in batches. Uses SCAN instead of KEYS to avoid blocking the server when
// the keyspace is large.
func wipeKeysByPrefix(ctx context.Context, c *cache.RedisCache, prefix string) (int64, error) {
	cli := c.Client()
	if cli == nil {
		return 0, nil
	}
	var (
		cursor  uint64
		deleted int64
	)
	for {
		keys, next, err := cli.Scan(ctx, cursor, prefix+"*", 256).Result()
		if err != nil {
			return deleted, err
		}
		if len(keys) > 0 {
			n, derr := cli.Del(ctx, keys...).Result()
			if derr != nil {
				return deleted, derr
			}
			deleted += n
		}
		cursor = next
		if cursor == 0 {
			return deleted, nil
		}
	}
}
