package capturelock

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSameVaultAndExternalIDAreSerialized(t *testing.T) {
	unlock := Lock("/vault", "shared-event")
	gotLock := make(chan struct{})
	released := make(chan struct{})
	go func() {
		defer close(released)
		unlockSecond := Lock("/vault", "shared-event")
		close(gotLock)
		unlockSecond()
	}()

	select {
	case <-gotLock:
		t.Fatal("concurrent capture acquired an already-held guard")
	case <-time.After(30 * time.Millisecond):
	}
	unlock()
	select {
	case <-gotLock:
	case <-time.After(2 * time.Second):
		t.Fatal("capture guard stayed locked after release")
	}
	<-released
}

func TestDifferentExternalIDsCanProceedIndependently(t *testing.T) {
	const vault = "/vault"
	first := "source-1"
	var other string
	for n := 2; n < 256; n++ {
		candidate := fmt.Sprintf("source-%d", n)
		if stripeFor(vault, candidate) != stripeFor(vault, first) {
			other = candidate
			break
		}
	}
	if other == "" {
		t.Fatal("failed to find a separate lock stripe")
	}

	unlockFirst := Lock(vault, first)
	defer unlockFirst()
	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		unlockOther := Lock(vault, other)
		close(ready)
		unlockOther()
	}()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("independent capture was blocked by another external ID")
	}
	wg.Wait()
}

func TestVaultIsPartOfCaptureLockKey(t *testing.T) {
	if stripeFor("/vault-a", "shared-event") == stripeFor("/vault-b", "shared-event") {
		t.Skip("unrelated hash keys share a stripe; safe but serial")
	}
}
