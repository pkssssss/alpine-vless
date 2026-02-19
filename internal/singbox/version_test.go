package singbox

import "testing"

func TestParseVersionOutput_MainPattern(t *testing.T) {
	t.Parallel()

	v, err := parseVersionOutput("sing-box version 1.11.4\n")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if v != "1.11.4" {
		t.Fatalf("expected 1.11.4, got %q", v)
	}
}

func TestParseVersionOutput_WithPrefixV(t *testing.T) {
	t.Parallel()

	v, err := parseVersionOutput("sing-box version v1.12.0")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if v != "1.12.0" {
		t.Fatalf("expected 1.12.0, got %q", v)
	}
}

func TestParseVersionOutput_Unsupported(t *testing.T) {
	t.Parallel()

	_, err := parseVersionOutput("hello world")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
