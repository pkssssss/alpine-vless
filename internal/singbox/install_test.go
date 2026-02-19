package singbox

import (
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
