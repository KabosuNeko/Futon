package storage

import (
	"os"
	"testing"
)

// TestMain redirects HOME to a temp dir for the whole test binary.
//
// SaveHistory (history.go) debounces disk writes on a 2s timer. A test that
// saves history can schedule the flush and finish before it fires; the late
// timer would then write test data into the developer's real ~/.config/futon/.
// Pinning HOME here keeps every delayed flush inside a throwaway temp dir.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "futon-storage-test-home-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
