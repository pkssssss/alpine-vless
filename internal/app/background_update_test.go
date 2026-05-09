package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkssssss/alpine-vless/internal/paths"
)

func TestUpdateSingBoxStartsBackgroundWorker(t *testing.T) {
	t.Parallel()

	p := updateTestPaths(t)
	var out bytes.Buffer
	started := false
	followed := false
	a := &App{
		Paths: p,
		Out:   &out,
		startUpdateWorker: func(spec updateWorkerSpec) (int, error) {
			started = true
			if spec.LogPath != p.UpdateLogPath {
				t.Fatalf("expected log path %q, got %q", p.UpdateLogPath, spec.LogPath)
			}
			return 1234, nil
		},
		followUpdateLog: func(ctx context.Context, logPath string, out io.Writer) error {
			followed = true
			if logPath != p.UpdateLogPath {
				t.Fatalf("expected followed log path %q, got %q", p.UpdateLogPath, logPath)
			}
			_, _ = fmt.Fprintln(out, "当前已是最新版本，无需更新。")
			return nil
		},
	}

	if err := a.UpdateSingBox(context.Background()); err != nil {
		t.Fatalf("expected background update start success, got %v", err)
	}

	if !started {
		t.Fatal("expected background worker to be started")
	}
	if !followed {
		t.Fatal("expected update log to be followed")
	}
	got := out.String()
	for _, want := range []string{
		"后台更新已启动",
		"PID: 1234",
		p.UpdateLogPath,
		"正在自动输出更新日志",
		"当前已是最新版本，无需更新。",
		"代理重启期间当前连接可能断开",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestShowUpdateStatusPrintsLogSummary(t *testing.T) {
	t.Parallel()

	p := updateTestPaths(t)
	if err := os.WriteFile(p.UpdateLogPath, []byte("开始更新 sing-box...\n当前版本: v1.13.11\n最新版本: v1.13.11\n当前已是最新版本，无需更新。\n"), 0600); err != nil {
		t.Fatalf("write update log: %v", err)
	}

	var out bytes.Buffer
	a := &App{
		Paths: p,
		Out:   &out,
	}

	if err := a.ShowUpdateStatus(context.Background()); err != nil {
		t.Fatalf("expected status success, got %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"更新状态: 已完成，无需更新",
		"日志文件: " + p.UpdateLogPath,
		"当前已是最新版本，无需更新。",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected status output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestNewUpdateWorkerCommandDetachesAndRedirectsLogs(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "update.log")
	cmd, cleanup, err := newUpdateWorkerCommand(updateWorkerSpec{
		ExePath: "/usr/local/bin/alpine-vless",
		LogPath: logFile,
	})
	if err != nil {
		t.Fatalf("expected command build success, got %v", err)
	}
	defer cleanup()

	if cmd.Path != "/usr/local/bin/alpine-vless" {
		t.Fatalf("expected worker path, got %q", cmd.Path)
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != UpdateWorkerArg {
		t.Fatalf("expected worker args to include %q, got %#v", UpdateWorkerArg, cmd.Args)
	}
	if cmd.Stdin == nil || cmd.Stdout == nil || cmd.Stderr == nil {
		t.Fatal("expected stdio to be detached from caller")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("expected worker process to start in a new session")
	}

	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("expected log file to be created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected log file mode 0600, got %v", info.Mode().Perm())
	}
}

func updateTestPaths(t *testing.T) paths.Paths {
	t.Helper()

	rootDir := t.TempDir()
	p := paths.Paths{
		RootDir:       rootDir,
		SingBoxPath:   filepath.Join(rootDir, "sing-box"),
		ConfigPath:    filepath.Join(rootDir, "config.json"),
		UpdateLogPath: filepath.Join(rootDir, "update.log"),
		ServiceName:   "alpine-vless",
		ServiceFile:   filepath.Join(rootDir, "alpine-vless.initd"),
	}

	if err := os.WriteFile(p.ConfigPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(p.SingBoxPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write sing-box: %v", err)
	}
	if err := os.WriteFile(p.ServiceFile, []byte("# managed-by: alpine-vless\n"), 0755); err != nil {
		t.Fatalf("write service file: %v", err)
	}
	return p
}
