package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPGetRetriesServerErrors(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	resp, err := httpGet(context.Background(), http.DefaultClient, srv.URL, "test")
	if err != nil {
		t.Fatalf("httpGet error: %v", err)
	}
	resp.Body.Close()
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (one retry)", attempts)
	}
}

func TestHTTPDoStopsRetryingWhenCancelled(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	if _, err := httpGet(ctx, http.DefaultClient, srv.URL, "test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry after cancel)", attempts)
	}
}

func TestNetworkErrorOmitsFullURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := srv.URL
	srv.Close() // nothing listens any more: connection refused

	_, err := httpGet(context.Background(), &http.Client{Timeout: time.Second}, addr+"/manga?availableTranslatedLanguage%5B%5D=vi&includes%5B%5D=cover_art", "test")
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if msg := err.Error(); strings.Contains(msg, "includes") || !strings.Contains(msg, strings.TrimPrefix(addr, "http://")) {
		t.Errorf("error should name the host, not the full URL: %q", msg)
	}
}
