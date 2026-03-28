package singbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
		hosts:  map[string]struct{}{"api.github.com": {}},
	}

	got, err := releaseAssetSHA256(context.Background(), httpClient, version, assetName)
	if err != nil {
		t.Fatalf("expected digest fallback success, got %v", err)
	}
	if got != sha {
		t.Fatalf("expected %s, got %s", sha, got)
	}
}

func TestInstall_PrefersMuslAssetWhenAvailable(t *testing.T) {
	t.Parallel()

	const (
		version          = "1.13.4"
		arch             = "amd64"
		genericAssetName = "sing-box-1.13.4-linux-amd64.tar.gz"
		muslAssetName    = "sing-box-1.13.4-linux-amd64-musl.tar.gz"
	)

	genericArchive := buildTarGzArchive(t, "sing-box-1.13.4-linux-amd64/sing-box", []byte("generic-binary"))
	muslArchive := buildTarGzArchive(t, "sing-box-1.13.4-linux-amd64-musl/sing-box", []byte("musl-binary"))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/SagerNet/sing-box/releases/tags/v"+version, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Assets []ReleaseAsset `json:"assets"`
		}{
			Assets: []ReleaseAsset{
				{
					Name:               genericAssetName,
					Digest:             "sha256:" + sha256Hex(genericArchive),
					BrowserDownloadURL: "https://github.com/SagerNet/sing-box/releases/download/v1.13.4/" + genericAssetName,
				},
				{
					Name:               muslAssetName,
					Digest:             "sha256:" + sha256Hex(muslArchive),
					BrowserDownloadURL: "https://github.com/SagerNet/sing-box/releases/download/v1.13.4/" + muslAssetName,
				},
			},
		})
	})
	mux.HandleFunc("/SagerNet/sing-box/releases/download/v1.13.4/"+genericAssetName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(genericArchive)
	})
	mux.HandleFunc("/SagerNet/sing-box/releases/download/v1.13.4/"+muslAssetName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(muslArchive)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	httpClient := server.Client()
	httpClient.Transport = rewriteHostTransport{
		base:   httpClient.Transport,
		target: server.URL,
		hosts: map[string]struct{}{
			"api.github.com": {},
			"github.com":     {},
		},
	}

	destPath := filepath.Join(t.TempDir(), "sing-box")
	err := Install(context.Background(), httpClient, InstallSpec{
		Version:  version,
		Arch:     arch,
		DestPath: destPath,
	})
	if err != nil {
		t.Fatalf("expected install success, got %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(got) != "musl-binary" {
		t.Fatalf("expected musl asset to be installed, got %q", string(got))
	}
}

func TestInstall_FallsBackToGenericAssetWhenMuslUnavailable(t *testing.T) {
	t.Parallel()

	const (
		version          = "1.12.22"
		arch             = "amd64"
		genericAssetName = "sing-box-1.12.22-linux-amd64.tar.gz"
	)

	genericArchive := buildTarGzArchive(t, "sing-box-1.12.22-linux-amd64/sing-box", []byte("generic-binary"))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/SagerNet/sing-box/releases/tags/v"+version, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Assets []ReleaseAsset `json:"assets"`
		}{
			Assets: []ReleaseAsset{
				{
					Name:               genericAssetName,
					Digest:             "sha256:" + sha256Hex(genericArchive),
					BrowserDownloadURL: "https://github.com/SagerNet/sing-box/releases/download/v1.12.22/" + genericAssetName,
				},
			},
		})
	})
	mux.HandleFunc("/SagerNet/sing-box/releases/download/v1.12.22/"+genericAssetName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(genericArchive)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	httpClient := server.Client()
	httpClient.Transport = rewriteHostTransport{
		base:   httpClient.Transport,
		target: server.URL,
		hosts: map[string]struct{}{
			"api.github.com": {},
			"github.com":     {},
		},
	}

	destPath := filepath.Join(t.TempDir(), "sing-box")
	err := Install(context.Background(), httpClient, InstallSpec{
		Version:  version,
		Arch:     arch,
		DestPath: destPath,
	})
	if err != nil {
		t.Fatalf("expected install success, got %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(got) != "generic-binary" {
		t.Fatalf("expected generic asset fallback, got %q", string(got))
	}
}

func TestExtractSingBoxBinary_SupportsMuslArchiveLayout(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "sing-box.tar.gz")
	archive := buildTarGzArchive(t, "sing-box-1.13.4-linux-amd64-musl/sing-box", []byte("musl-binary"))
	if err := os.WriteFile(archivePath, archive, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "sing-box")
	if err := extractSingBoxBinary(archivePath, "sing-box-1.13.4-linux-amd64-musl.tar.gz", destPath); err != nil {
		t.Fatalf("expected extract success, got %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(got) != "musl-binary" {
		t.Fatalf("expected musl binary content, got %q", string(got))
	}
}

type rewriteHostTransport struct {
	base   http.RoundTripper
	target string
	hosts  map[string]struct{}
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, ok := t.hosts[req.URL.Host]; !ok {
		return t.base.RoundTrip(req)
	}

	targetReq := req.Clone(req.Context())
	targetReq.URL.Scheme = "http"
	targetReq.URL.Host = strings.TrimPrefix(t.target, "http://")
	targetReq.Host = targetReq.URL.Host
	return t.base.RoundTrip(targetReq)
}

func buildTarGzArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
