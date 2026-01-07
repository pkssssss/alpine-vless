package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

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
	arch, err := singbox.DetectArch(runtime.GOARCH)
	if err != nil {
		return err
	}

	if err := system.MkdirAll0700(a.Paths.RootDir); err != nil {
		return err
	}

	version, err := singbox.LatestVersion(ctx, a.httpClient)
	if err != nil {
		return err
	}

	if err := singbox.Install(ctx, a.httpClient, singbox.InstallSpec{
		Version:  version,
		Arch:     arch,
		DestPath: a.Paths.SingBoxPath,
	}); err != nil {
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

	fmt.Fprintln(a.Out, "已生成并部署完成（单节点，覆盖式）。")
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
