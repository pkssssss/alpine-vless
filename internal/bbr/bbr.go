package bbr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkssssss/alpine-vless/internal/system"
)

type Result struct {
	AlreadyEnabled     bool
	CongestionControl  string
	DefaultQdisc       string
	AvailableAlgorithms string
}

const (
	managedMarker = "# managed-by: alpine-vless"

	sysctlConfDir  = "/etc/sysctl.d"
	sysctlConfFile = "/etc/sysctl.d/99-alpine-vless-bbr.conf"

	modulesFile = "/etc/modules"
)

func Enable(ctx context.Context) (Result, error) {
	if !system.CommandExists("sysctl") {
		return Result{}, errors.New("未找到 sysctl 命令")
	}

	curCC, _ := readProc("/proc/sys/net/ipv4/tcp_congestion_control")
	curQdisc, _ := readProc("/proc/sys/net/core/default_qdisc")
	already := curCC == "bbr" && curQdisc == "fq"

	if system.CommandExists("modprobe") {
		_ = system.Run(ctx, "modprobe", "sch_fq")
		_ = system.Run(ctx, "modprobe", "tcp_bbr")
	}

	avail, _ := readProc("/proc/sys/net/ipv4/tcp_available_congestion_control")
	if avail != "" && !containsWord(avail, "bbr") {
		return Result{}, fmt.Errorf("当前内核未提供 bbr（tcp_available_congestion_control=%q），可能需要安装/加载 tcp_bbr 内核模块或更换支持 BBR 的内核", avail)
	}

	if err := system.Run(ctx, "sysctl", "-w", "net.core.default_qdisc=fq"); err != nil {
		return Result{}, err
	}
	if err := system.Run(ctx, "sysctl", "-w", "net.ipv4.tcp_congestion_control=bbr"); err != nil {
		return Result{}, err
	}

	cc, err := readProc("/proc/sys/net/ipv4/tcp_congestion_control")
	if err != nil {
		return Result{}, err
	}
	qdisc, err := readProc("/proc/sys/net/core/default_qdisc")
	if err != nil {
		return Result{}, err
	}
	if cc != "bbr" || qdisc != "fq" {
		return Result{}, fmt.Errorf("BBR 开启未生效：tcp_congestion_control=%q default_qdisc=%q", cc, qdisc)
	}

	if err := ensureSysctlPersist(); err != nil {
		return Result{}, err
	}
	if err := ensureModulesPersist(); err != nil {
		return Result{}, err
	}

	_ = system.Run(ctx, "sysctl", "-p", sysctlConfFile)

	if system.CommandExists("rc-update") {
		_ = system.Run(ctx, "rc-update", "add", "sysctl", "boot")
		_ = system.Run(ctx, "rc-update", "add", "modules", "boot")
	}
	if system.CommandExists("rc-service") {
		_ = system.Run(ctx, "rc-service", "sysctl", "restart")
		_ = system.Run(ctx, "rc-service", "modules", "restart")
	}

	return Result{
		AlreadyEnabled:     already,
		CongestionControl:  cc,
		DefaultQdisc:       qdisc,
		AvailableAlgorithms: avail,
	}, nil
}

func ensureSysctlPersist() error {
	if err := os.MkdirAll(sysctlConfDir, 0755); err != nil {
		return err
	}

	if b, err := os.ReadFile(sysctlConfFile); err == nil {
		if !strings.Contains(string(b), managedMarker) {
			return fmt.Errorf("检测到已有 sysctl 配置文件 %s，但不是本工具管理，拒绝覆盖", sysctlConfFile)
		}
	}

	content := strings.TrimLeft(fmt.Sprintf(`%s
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
`, managedMarker), "\n")
	return os.WriteFile(sysctlConfFile, []byte(content), 0644)
}

func ensureModulesPersist() error {
	if err := os.MkdirAll(filepath.Dir(modulesFile), 0755); err != nil {
		return err
	}

	b, err := os.ReadFile(modulesFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.WriteFile(modulesFile, []byte("tcp_bbr\n"), 0644)
		}
		return err
	}

	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if t == "tcp_bbr" || strings.HasPrefix(t, "tcp_bbr ") {
			return nil
		}
	}

	f, err := os.OpenFile(modulesFile, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(b) > 0 && b[len(b)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString("tcp_bbr\n")
	return err
}

func readProc(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func containsWord(s, word string) bool {
	for _, f := range strings.Fields(s) {
		if f == word {
			return true
		}
	}
	return false
}
