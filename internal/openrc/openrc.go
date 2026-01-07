package openrc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkssssss/alpine-vless/internal/paths"
	"github.com/pkssssss/alpine-vless/internal/system"
)

const managedMarker = "# managed-by: alpine-vless"

const (
	legacyServiceName = "sing-box"
	legacyServiceFile = "/etc/init.d/sing-box"
)

func IsManagedServiceFile(serviceFile string) bool {
	b, err := os.ReadFile(serviceFile)
	if err != nil {
		return false
	}
	return bytes.Contains(b, []byte(managedMarker))
}

func InstallServiceFile(p paths.Paths) error {
	if b, err := os.ReadFile(p.ServiceFile); err == nil {
		if !bytes.Contains(b, []byte(managedMarker)) {
			return fmt.Errorf("检测到已有服务文件 %s，但不是本工具管理，拒绝覆盖", p.ServiceFile)
		}
	}

	pidfile := filepath.Join("/run", p.ServiceName+".pid")
	content := strings.TrimLeft(fmt.Sprintf(`#!/sbin/openrc-run
%s
command="%s"
command_args="run -c \"%s\""
command_background=yes
pidfile="%s"
output_log="%s"
error_log="%s"

depend() {
    need net
}
`, managedMarker, p.SingBoxPath, p.ConfigPath, pidfile, p.OpenRCOutLogPath, p.OpenRCErrLogPath), "\n")

	if err := os.WriteFile(p.ServiceFile, []byte(content), 0755); err != nil {
		return err
	}
	return os.Chmod(p.ServiceFile, 0755)
}

func EnableAndStart(ctx context.Context, serviceName string) error {
	_ = system.Run(ctx, "rc-update", "add", serviceName, "default")
	if err := system.Run(ctx, "rc-service", serviceName, "restart"); err == nil {
		return nil
	}
	return system.Run(ctx, "rc-service", serviceName, "start")
}

func CleanupLegacyManaged(ctx context.Context) error {
	if !system.FileExists(legacyServiceFile) || !IsManagedServiceFile(legacyServiceFile) {
		return nil
	}
	_ = system.Run(ctx, "rc-service", legacyServiceName, "stop")
	_ = system.Run(ctx, "rc-update", "del", legacyServiceName, "default")
	_ = os.Remove(legacyServiceFile)
	return nil
}

func StopDisableAndRemove(ctx context.Context, p paths.Paths) error {
	if system.FileExists(p.ServiceFile) && !IsManagedServiceFile(p.ServiceFile) {
		return errors.New("检测到非本工具管理的 OpenRC 服务文件，拒绝卸载")
	}

	_ = system.Run(ctx, "rc-service", p.ServiceName, "stop")
	_ = system.Run(ctx, "rc-update", "del", p.ServiceName, "default")
	_ = os.Remove(p.ServiceFile)

	_ = CleanupLegacyManaged(ctx)
	return nil
}
