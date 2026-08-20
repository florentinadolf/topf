## Bug: `*decryption.Cache` is unusable by external callers, causing a nil pointer panic

### Summary

`decryption.Cache` (used by `config.LoadFromFile`, `config.(*TopfConfig).GetSecretsProvider`,
`config.PatchesConfig.DecryptCache`, and `providers.NewFilesystemSecretsProvider`) lives in
`internal/decryption`. Because it's an internal package, external modules that depend on
`github.com/postfinance/topf` (e.g. `pkg/config`, `pkg/providers` consumers) cannot import it
and therefore cannot construct a `*decryption.Cache`.

The only value an external caller can pass to these exported APIs is `nil`. But
`Cache.ReadFile` dereferences its receiver unconditionally:

```go
func (c *Cache) ReadFile(path string) ([]byte, []string, error) {
	c.mu.RLock()   // panic: nil pointer dereference when c == nil
	...
}
```

So any external consumer calling `config.LoadFromFile(path, nil)` (the only option available
to them) panics at runtime with a nil pointer dereference.

### Affected version

`v0.5.0` (commit `2b367d9`).

### Affected exported API surface

| File | Symbol |
|---|---|
| `pkg/config/topf_config.go:63` | `LoadFromFile(path string, cache *decryption.Cache)` |
| `pkg/config/topf_config.go:137` | `(*TopfConfig).GetSecretsProvider(cache *decryption.Cache)` |
| `pkg/config/patches.go:34` | `PatchContext.DecryptCache *decryption.Cache` |
| `pkg/providers/secrets_filesystem.go:17` | `NewFilesystemSecretsProvider(secretsPath string, cache *decryption.Cache)` |

All four take/require a `*decryption.Cache`, but `decryption.Cache` cannot be constructed
outside this module since `internal/decryption` is not importable externally.

### Repro

```go
package main

import "github.com/postfinance/topf/pkg/config"

func main() {
	// nil is the only *decryption.Cache value available to an external caller
	_, _, err := config.LoadFromFile("topf.yaml", nil)
	if err != nil {
		panic(err)
	}
}
```

Result: panic inside `Cache.ReadFile` at `c.mu.RLock()`, `c` is `nil`.

### Suggested fix

Make `*Cache` nil-safe rather than exposing/moving the package (keeping `decryption` internal
is intentional/preferred). Since every consumer of the cache funnels through
`Cache.ReadFile`, guarding that one method fixes all four call sites:

```go
// A nil *Cache is valid and behaves as a cache that never stores anything:
// every ReadFile call performs a fresh read.
func (c *Cache) ReadFile(path string) ([]byte, []string, error) {
	if c == nil {
		return readAndDecrypt(path) // extracted read/decrypt/vals-eval logic, no caching
	}
	// ... existing cached/singleflight path unchanged
}
```

This preserves all existing behavior (caching, dedup via singleflight) when a real `*Cache` is
supplied, and makes `nil` a fully valid, documented value: it just disables caching/dedup for
that call. Doc comments on the four exported APIs above should note that `cache`/`DecryptCache`
may be `nil`.

### Suggested tests

Add to `internal/decryption/decryption_test.go`:
- `ReadFile` on a `nil *Cache` returns correct content/secrets for a plain file, called twice
  (proves no panic and no reliance on cache state).
- `ReadFile` on a `nil *Cache` returns `fs.ErrNotExist`-wrapped error for a missing file.

### Why this matters

Any downstream module using `pkg/config.LoadFromFile` or `pkg/providers.NewFilesystemSecretsProvider`
without vendoring/copying `internal/decryption` (impossible, by design) will panic in production
the first time these code paths run. This effectively makes `pkg/config` and `pkg/providers`
unusable as a public API without triggering this panic.
