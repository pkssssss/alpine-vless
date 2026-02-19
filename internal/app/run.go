package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/pkssssss/alpine-vless/internal/bbr"
	"github.com/pkssssss/alpine-vless/internal/menu"
	"github.com/pkssssss/alpine-vless/internal/openrc"
	"github.com/pkssssss/alpine-vless/internal/paths"
	"github.com/pkssssss/alpine-vless/internal/singbox"
	"github.com/pkssssss/alpine-vless/internal/system"
)

type App struct {
	Paths paths.Paths
	Out   io.Writer
	Err   io.Writer

	httpClient *http.Client
}

func Run(ctx context.Context, in io.Reader, out, errOut io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("仅支持在 Linux（Alpine）运行")
	}
	if !system.IsRoot() {
		return errors.New("需要 root 权限运行")
	}
	if !system.IsAlpine() {
		return errors.New("仅支持 Alpine Linux")
	}
	if !system.CommandExists("rc-service") || !system.CommandExists("rc-update") {
		return errors.New("未检测到 OpenRC（缺少 rc-service/rc-update）")
	}

	p, err := paths.Discover()
	if err != nil {
		return err
	}

	a := &App{
		Paths: p,
		Out:   out,
		Err:   errOut,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if !system.FileExists(a.Paths.ConfigPath) {
		fmt.Fprintln(a.Out, "未检测到已部署实例，开始自动安装并生成配置...")
		if err := a.Add(ctx); err != nil {
			return err
		}
	} else if !a.IsInstalled() {
		fmt.Fprintln(a.Out, "检测到已有配置，但 OpenRC 服务未安装或非本工具管理；可从菜单选择“添加配置(覆盖)”或“卸载”。")
	}

	return menu.Run(ctx, bufio.NewReader(in), out, errOut, a)
}

func (a *App) IsInstalled() bool {
	if !system.FileExists(a.Paths.ConfigPath) {
		return false
	}
	if !system.FileExists(a.Paths.SingBoxPath) {
		return false
	}
	return openrc.IsManagedServiceFile(a.Paths.ServiceFile)
}

func (a *App) Add(ctx context.Context) error {
	version, err := a.installLatestSingBox(ctx)
	if err != nil {
		return err
	}

	node, err := singbox.NewDefaultNode(ctx)
	if err != nil {
		return err
	}

	if err := singbox.WriteConfig(a.Paths.ConfigPath, a.Paths.LogPath, node); err != nil {
		return err
	}

	if err := singbox.CheckConfig(ctx, a.Paths.SingBoxPath, a.Paths.ConfigPath); err != nil {
		return err
	}

	if err := openrc.CleanupLegacyManaged(ctx); err != nil {
		return err
	}
	if err := openrc.InstallServiceFile(a.Paths); err != nil {
		return err
	}
	if err := openrc.EnableAndStart(ctx, a.Paths.ServiceName); err != nil {
		return err
	}

	ip, _ := singbox.PublicIP(ctx, a.httpClient)
	pub, err := singbox.RealityPublicKeyFromPrivateKey(node.RealityPrivateKey)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.Out, "已生成并部署完成（单节点，覆盖式，sing-box v%s）。\n", version)
	fmt.Fprintln(a.Out, node.URL(ip, pub))
	return nil
}

func (a *App) Show(ctx context.Context) error {
	cfg, err := singbox.ReadConfig(a.Paths.ConfigPath)
	if err != nil {
		return err
	}

	ip, _ := singbox.PublicIP(ctx, a.httpClient)
	pub, err := singbox.RealityPublicKeyFromPrivateKey(cfg.Node.RealityPrivateKey)
	if err != nil {
		return err
	}

	fmt.Fprintln(a.Out, cfg.Node.URL(ip, pub))
	return nil
}

func (a *App) Uninstall(ctx context.Context) error {
	if err := openrc.StopDisableAndRemove(ctx, a.Paths); err != nil {
		return err
	}
	if err := system.RemoveAll(a.Paths.RootDir); err != nil {
		return err
	}

	fmt.Fprintln(a.Out, "卸载完成，已移除服务与所有落地文件。")
	return nil
}

func (a *App) EnableBBR(ctx context.Context) error {
	res, err := bbr.Enable(ctx)
	if err != nil {
		return err
	}

	if res.AlreadyEnabled {
		fmt.Fprintln(a.Out, "BBR 已处于开启状态。")
	} else {
		fmt.Fprintln(a.Out, "BBR 已开启并写入持久化配置。")
	}
	fmt.Fprintf(a.Out, "当前拥塞控制算法: %s\n", res.CongestionControl)
	fmt.Fprintf(a.Out, "当前 default_qdisc: %s\n", res.DefaultQdisc)
	return nil
}

func (a *App) UpdateSingBox(ctx context.Context) error {
	if !system.FileExists(a.Paths.ConfigPath) {
		return errors.New("未检测到已部署配置，请先执行“添加配置（重生成/覆盖）”")
	}
	if !system.FileExists(a.Paths.SingBoxPath) {
		return errors.New("未检测到当前 sing-box 可执行文件，请先执行“添加配置（重生成/覆盖）”")
	}
	if !openrc.IsManagedServiceFile(a.Paths.ServiceFile) {
		return errors.New("未检测到本工具管理的 OpenRC 服务，请先执行“添加配置（重生成/覆盖）”")
	}

	nextPath := a.Paths.SingBoxPath + ".new"
	backupPath := a.Paths.SingBoxPath + ".bak"
	if err := os.Remove(nextPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	defer func() { _ = os.Remove(nextPath) }()

	version, err := a.installLatestSingBoxTo(ctx, nextPath)
	if err != nil {
		return err
	}
	if err := singbox.CheckConfig(ctx, nextPath, a.Paths.ConfigPath); err != nil {
		return fmt.Errorf("新版本配置校验失败，已取消替换: %w", err)
	}
	if err := swapBinaryForUpdate(nextPath, a.Paths.SingBoxPath, backupPath); err != nil {
		return err
	}
	if err := openrc.EnableAndStart(ctx, a.Paths.ServiceName); err != nil {
		if rollbackErr := rollbackBinary(a.Paths.SingBoxPath, backupPath); rollbackErr != nil {
			return fmt.Errorf("更新后重启失败，且回滚失败: 重启错误=%v, 回滚错误=%w", err, rollbackErr)
		}
		if restartErr := openrc.EnableAndStart(ctx, a.Paths.ServiceName); restartErr != nil {
			return fmt.Errorf("更新后重启失败，已回滚旧版本但服务恢复失败: 重启错误=%v, 恢复错误=%w", err, restartErr)
		}
		return fmt.Errorf("更新后重启失败，已自动回滚到旧版本: %w", err)
	}
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(a.Err, "警告: 清理备份文件失败：%v\n", err)
	}

	fmt.Fprintf(a.Out, "sing-box 已更新到 v%s，并已重启服务。\n", version)
	return nil
}

func (a *App) installLatestSingBox(ctx context.Context) (string, error) {
	return a.installLatestSingBoxTo(ctx, a.Paths.SingBoxPath)
}

func (a *App) installLatestSingBoxTo(ctx context.Context, destPath string) (string, error) {
	arch, err := singbox.DetectArch(runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if err := system.MkdirAll0700(a.Paths.RootDir); err != nil {
		return "", err
	}

	version, err := singbox.LatestVersion(ctx, a.httpClient)
	if err != nil {
		return "", err
	}

	if err := singbox.Install(ctx, a.httpClient, singbox.InstallSpec{
		Version:  version,
		Arch:     arch,
		DestPath: destPath,
	}); err != nil {
		return "", err
	}

	return version, nil
}

func swapBinaryForUpdate(newPath, destPath, backupPath string) error {
	if err := os.Rename(destPath, backupPath); err != nil {
		return fmt.Errorf("备份旧版本失败: %w", err)
	}
	if err := os.Rename(newPath, destPath); err != nil {
		if restoreErr := os.Rename(backupPath, destPath); restoreErr != nil {
			return fmt.Errorf("替换新版本失败且恢复旧版本失败: 替换错误=%v, 恢复错误=%w", err, restoreErr)
		}
		return fmt.Errorf("替换新版本失败，已恢复旧版本: %w", err)
	}
	return nil
}

func rollbackBinary(destPath, backupPath string) error {
	if err := os.Remove(destPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("回滚前清理当前二进制失败: %w", err)
	}
	if err := os.Rename(backupPath, destPath); err != nil {
		return fmt.Errorf("回滚旧版本失败: %w", err)
	}
	return nil
}
