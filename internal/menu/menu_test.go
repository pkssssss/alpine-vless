package menu

import (
	"bufio"
	"bytes"
	"context"
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

func TestRunShowsUpdateStatusFromMenu(t *testing.T) {
	t.Parallel()

	in := bufio.NewReader(strings.NewReader("6\n0\n"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	h := &fakeHandler{}

	if err := Run(context.Background(), in, &out, &errOut, h); err != nil {
		t.Fatalf("expected menu run success, got %v", err)
	}

	if !h.showUpdateStatusCalled {
		t.Fatal("expected option 6 to call ShowUpdateStatus")
	}
	if !strings.Contains(out.String(), "6) 查看更新状态/日志") {
		t.Fatalf("expected menu to show update status option, got:\n%s", out.String())
	}
}

type fakeHandler struct {
	showUpdateStatusCalled bool
}

func (h *fakeHandler) Add(context.Context) error           { return nil }
func (h *fakeHandler) Show(context.Context) error          { return nil }
func (h *fakeHandler) Uninstall(context.Context) error     { return nil }
func (h *fakeHandler) EnableBBR(context.Context) error     { return nil }
func (h *fakeHandler) UpdateSingBox(context.Context) error { return nil }
func (h *fakeHandler) ShowUpdateStatus(context.Context) error {
	h.showUpdateStatusCalled = true
	return nil
}
