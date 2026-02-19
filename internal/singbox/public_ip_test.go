package singbox

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPublicIP_OK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "1.2.3.4\n")
	}))
	defer srv.Close()

	ip, err := fetchPublicIP(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %q", ip)
	}
}

func TestFetchPublicIP_Non2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, "rate limit")
	}))
	defer srv.Close()

	_, err := fetchPublicIP(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("expected HTTP 429 in error, got %v", err)
	}
}

func TestFetchPublicIP_InvalidBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not-an-ip")
	}))
	defer srv.Close()

	_, err := fetchPublicIP(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "不是合法 IP") {
		t.Fatalf("expected invalid IP error, got %v", err)
	}
}
