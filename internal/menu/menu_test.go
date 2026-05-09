package menu

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestConfirmUpdateSingBoxWarnsWhenConnectionDependsOnNode(t *testing.T) {
	t.Parallel()

	in := bufio.NewReader(strings.NewReader("no\n"))
	var out bytes.Buffer

	if confirmUpdateSingBox(in, &out) {
		t.Fatal("expected update to be cancelled")
	}
	if !strings.Contains(out.String(), "如果当前远程连接依赖本节点") {
		t.Fatalf("expected warning about dependent remote connections, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "后台更新任务") {
		t.Fatalf("expected background update wording, got:\n%s", out.String())
	}
}
