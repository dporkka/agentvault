// Package capturelock serializes idempotent capture attempts in this Go
// process. It is not a cross-process lock or a durable idempotency ledger.
package capturelock

import (
	"hash/fnv"
	"path/filepath"
	"sync"
)

// A fixed number of stripes avoids unbounded maps of user-provided IDs.
// A hash collision only serializes independent captures; it never merges IDs.
const stripeCount = 128

var stripes [stripeCount]sync.Mutex

// Lock acquires the process-local guard for a vault and external capture ID.
// Callers must hold it across the existing-file check AND final publication,
// and invoke the returned release function exactly once (prefer defer).
// Unrelated external IDs generally run concurrently.
func Lock(vaultPath, externalID string) func() {
	lock := stripeFor(vaultPath, externalID)
	lock.Lock()
	return lock.Unlock
}

func stripeFor(vaultPath, externalID string) *sync.Mutex {
	h := fnv.New64a()
	_, _ = h.Write([]byte(filepath.Clean(vaultPath)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(externalID))
	return &stripes[h.Sum64()%stripeCount]
}
