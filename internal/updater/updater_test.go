package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestCheckForUpdate_dev(t *testing.T) {
	ok, tag, url, err := CheckForUpdate("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for dev version")
	}
	if tag != "" || url != "" {
		t.Fatal("expected empty tag and url for dev version")
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

	ok, _, _, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false when versions are equal")
	}
}

func TestCheckForUpdate_hasUpdate(t *testing.T) {
	tag := "v2.0.0"
	ver := strings.TrimPrefix(tag, "v")
	wanted := fmt.Sprintf("futon_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{
			TagName: tag,
			Assets: []releaseAsset{
				{Name: "other_file", BrowserDownloadURL: "https://example.com/other"},
				{Name: wanted, BrowserDownloadURL: "https://example.com/futon.tar.gz"},
			},
		})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	ok, gotTag, gotURL, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true when update available")
	}
	if gotTag != tag {
		t.Fatalf("expected tag %s, got %s", tag, gotTag)
	}
	if gotURL != "https://example.com/futon.tar.gz" {
		t.Fatalf("expected URL https://example.com/futon.tar.gz, got %s", gotURL)
	}
}

func TestCheckForUpdate_noMatchingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{
			TagName: "v2.0.0",
			Assets:  []releaseAsset{{Name: "wrong_name.tar.gz", BrowserDownloadURL: "https://example.com/wrong"}},
		})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	_, _, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error when no matching asset")
	}
	if !strings.Contains(err.Error(), "no asset found") {
		t.Fatalf("expected 'no asset found', got: %v", err)
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

	_, _, _, err := CheckForUpdate("v1.0.0")
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

	_, _, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}
