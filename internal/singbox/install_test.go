package singbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChecksumLine_GNUStyle(t *testing.T) {
	t.Parallel()

	asset := "sing-box-1.11.0-linux-amd64.tar.gz"
	sha := strings.Repeat("a", 64)
	line := sha + "  " + asset

	got, ok := parseChecksumLine(line, asset)
	if !ok {
		t.Fatal("expected parse success")
	}
	if got != sha {
		t.Fatalf("expected %s, got %s", sha, got)
	}
}

func TestParseChecksumLine_OpenSSLStyle(t *testing.T) {
	t.Parallel()

	asset := "sing-box-1.11.0-linux-amd64.tar.gz"
	sha := strings.Repeat("b", 64)
	line := "SHA256 (" + asset + ") = " + sha

	got, ok := parseChecksumLine(line, asset)
	if !ok {
		t.Fatal("expected parse success")
	}
	if got != sha {
		t.Fatalf("expected %s, got %s", sha, got)
	}
}

func TestParseSHA256FromChecksums_NotFound(t *testing.T) {
	t.Parallel()

	_, err := parseSHA256FromChecksums("abcd  other.tar.gz", "target.tar.gz")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("abc"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := verifyFileSHA256(p, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"); err != nil {
		t.Fatalf("expected verify success, got %v", err)
	}
}

func TestReleaseAssetSHA256_UsesAssetDigestWhenChecksumAssetMissing(t *testing.T) {
	t.Parallel()

	const (
		version   = "1.13.4"
		assetName = "sing-box-1.13.4-linux-amd64.tar.gz"
		sha       = "634a679fc572d9d0c01b2f5f43b9d6af3f529e9f7011bdfc5931804fc0fa968a"
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/SagerNet/sing-box/releases/tags/v"+version, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Assets []ReleaseAsset `json:"assets"`
		}{
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Digest:             "sha256:" + sha,
					BrowserDownloadURL: "https://example.invalid/" + assetName,
				},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	httpClient := server.Client()
	httpClient.Transport = rewriteHostTransport{
		base:   httpClient.Transport,
		target: server.URL,
		host:   "api.github.com",
	}

	got, err := releaseAssetSHA256(context.Background(), httpClient, version, assetName)
	if err != nil {
		t.Fatalf("expected digest fallback success, got %v", err)
	}
	if got != sha {
		t.Fatalf("expected %s, got %s", sha, got)
	}
}

type rewriteHostTransport struct {
	base   http.RoundTripper
	target string
	host   string
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != t.host {
		return t.base.RoundTrip(req)
	}

	targetReq := req.Clone(req.Context())
	targetReq.URL.Scheme = "http"
	targetReq.URL.Host = strings.TrimPrefix(t.target, "http://")
	targetReq.Host = targetReq.URL.Host
	return t.base.RoundTrip(targetReq)
}
