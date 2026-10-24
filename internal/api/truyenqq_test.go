package api

import (
	"errors"
	"net/http"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTruyenQQConstructorDoesNotTouchNetwork(t *testing.T) {
	old := http.DefaultTransport
	probes := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		probes++
		return nil, errors.New("no network in test")
	})
	defer func() { http.DefaultTransport = old }()

	_ = NewTruyenQQProvider()
	if probes != 0 {
		t.Fatalf("constructor made %d network requests, want 0", probes)
	}
}

func TestTruyenQQDomainProbeIsLazyAndRunsOnce(t *testing.T) {
	old := http.DefaultTransport
	probes := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		probes++
		return nil, errors.New("no network in test")
	})
	defer func() { http.DefaultTransport = old }()

	p := NewTruyenQQProvider()
	if p.baseURL != truyenqqPrimaryURL {
		t.Fatalf("baseURL = %q before first request, want %q", p.baseURL, truyenqqPrimaryURL)
	}

	p.resolveDomain()
	if probes != 2 {
		t.Fatalf("probe requests = %d, want primary + fallback (2)", probes)
	}
	if p.baseURL != truyenqqPrimaryURL {
		t.Fatalf("baseURL = %q after failed probes, want primary", p.baseURL)
	}

	p.resolveDomain()
	if probes != 2 {
		t.Fatalf("probe requests = %d after second call, want 2 (sync.Once)", probes)
	}
}
