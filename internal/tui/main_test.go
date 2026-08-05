package tui

import (
	"os"
	"testing"
)

// TestMain redirects HOME to a temp dir for the whole test binary.
//
// storage.SaveHistory debounces disk writes on a 2s timer. A test may
// schedule that timer and end before it fires; once the test's t.Setenv
// restores the real HOME, the late flush would write test data into the
// developer's real ~/.config/futon/history.json. Pinning HOME here keeps
// every delayed flush inside a throwaway temp dir.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "futon-tui-test-home-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
