package app

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPClientDoesNotUseWholeRequestTimeout(t *testing.T) {
	t.Parallel()

	client := newHTTPClient()

	if client.Timeout != 0 {
		t.Fatalf("expected no whole-request timeout for large downloads, got %s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout <= 0 {
		t.Fatal("expected response header timeout to protect API requests")
	}
	if transport.TLSHandshakeTimeout <= 0 {
		t.Fatal("expected TLS handshake timeout")
	}
	if transport.IdleConnTimeout <= 0 {
		t.Fatal("expected idle connection timeout")
	}
	if transport.ExpectContinueTimeout <= 0 {
		t.Fatal("expected expect-continue timeout")
	}
}

func TestDownloadTimeoutAllowsSlowReleaseAssets(t *testing.T) {
	t.Parallel()

	if downloadTimeout < 10*time.Minute {
		t.Fatalf("download timeout is too short for slow VPS GitHub downloads: %s", downloadTimeout)
	}
}
