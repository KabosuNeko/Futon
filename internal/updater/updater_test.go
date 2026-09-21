package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckForUpdate_dev(t *testing.T) {
	ok, tag, err := CheckForUpdate("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for dev version")
	}
	if tag != "" {
		t.Fatal("expected empty tag for dev version")
	}
}

func TestCheckForUpdate_noNewVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{TagName: "v1.0.0"})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	ok, _, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false when versions are equal")
	}
}

func TestCheckForUpdate_hasUpdate(t *testing.T) {
	tag := "v2.0.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{TagName: tag})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	ok, gotTag, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true when update available")
	}
	if gotTag != tag {
		t.Fatalf("expected tag %s, got %s", tag, gotTag)
	}
}

func TestCheckForUpdate_emptyTagNoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	ok, _, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false when release tag is empty")
	}
}

func TestCheckForUpdate_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	_, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error on non-200 status")
	}
	if !strings.Contains(err.Error(), "HTTP status: 404") {
		t.Fatalf("expected 404 error, got: %v", err)
	}
}

func TestCheckForUpdate_invalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{invalid json}`))
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	_, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestVersLE(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "1.0.0", true},
		{"1.0.1", "1.0.0", false},
		{"1.0", "1.0.0", true},
		{"1.2", "1.10", true},
		{"1.10", "1.2", false},
		{"2.0.0", "1.9.9", false},
	}
	for _, c := range cases {
		if got := versLE(c.a, c.b); got != c.want {
			t.Errorf("versLE(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestInstallScriptCommand(t *testing.T) {
	c := InstallScriptCommand()
	if c == nil {
		t.Fatal("expected a command")
	}
	if c.Args[0] != "bash" || c.Args[1] != "-c" {
		t.Fatalf("unexpected argv: %v", c.Args)
	}
	if !strings.Contains(c.Args[2], "install.sh") || !strings.Contains(c.Args[2], "/tmp/futon_install.sh") {
		t.Fatalf("unexpected install script command: %q", c.Args[2])
	}
}
