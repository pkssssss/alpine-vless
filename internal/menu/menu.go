package menu

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type Handler interface {
	Add(ctx context.Context) error
	Show(ctx context.Context) error
	Uninstall(ctx context.Context) error
	EnableBBR(ctx context.Context) error
}

func Run(ctx context.Context, in *bufio.Reader, out, errOut io.Writer, h Handler) error {
	for {
		fmt.Fprintln(out, "===== sing-box (VLESS Reality) =====")
		fmt.Fprintln(out, "1) 添加配置（重生成/覆盖）")
		fmt.Fprintln(out, "2) 查看配置（输出一键导入 URL）")
		fmt.Fprintln(out, "3) 删除配置（卸载/清空）")
		fmt.Fprintln(out, "4) 一键开启 BBR（fq + bbr）")
		fmt.Fprintln(out, "0) 退出")
		fmt.Fprint(out, "选择: ")

		line, err := in.ReadString('\n')
		if err != nil {
			return err
		}
		switch strings.TrimSpace(line) {
		case "1":
			if err := h.Add(ctx); err != nil {
				fmt.Fprintln(errOut, "错误:", err.Error())
			}
		case "2":
			if err := h.Show(ctx); err != nil {
				fmt.Fprintln(errOut, "错误:", err.Error())
			}
		case "3":
			if !confirmUninstall(in, out) {
				continue
			}
			if err := h.Uninstall(ctx); err != nil {
				fmt.Fprintln(errOut, "错误:", err.Error())
				continue
			}
			return nil
		case "4":
			if !confirmEnableBBR(in, out) {
				continue
			}
			if err := h.EnableBBR(ctx); err != nil {
				fmt.Fprintln(errOut, "错误:", err.Error())
			}
		case "0":
			return nil
		default:
			fmt.Fprintln(out, "无效选择")
		}
	}
}

func confirmUninstall(in *bufio.Reader, out io.Writer) bool {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "⚠️ 危险操作检测！")
	fmt.Fprintln(out, "操作类型：卸载（停止服务、移除 OpenRC、自启配置、删除落地文件）")
	fmt.Fprintln(out, "影响范围：当前工具管理的 sing-box 相关文件与服务")
	fmt.Fprintln(out, "风险评估：卸载后代理不可用，需要重新运行并安装")
	fmt.Fprintln(out)
	fmt.Fprint(out, "请确认是否继续？输入“确认卸载”继续: ")

	line, err := in.ReadString('\n')
	if err != nil {
		fmt.Fprintln(out, "读取输入失败，已取消。")
		return false
	}
	if strings.TrimSpace(line) != "确认卸载" {
		fmt.Fprintln(out, "已取消卸载。")
		return false
	}
	return true
}

func confirmEnableBBR(in *bufio.Reader, out io.Writer) bool {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "⚠️ 危险操作检测！")
	fmt.Fprintln(out, "操作类型：开启 BBR（修改 sysctl、可能加载内核模块、写入开机持久化配置）")
	fmt.Fprintln(out, "影响范围：系统网络栈全局参数（对所有 TCP 连接生效）")
	fmt.Fprintln(out, "风险评估：部分内核/机型不支持或与现有调参冲突，可能导致网络异常")
	fmt.Fprintln(out)
	fmt.Fprint(out, "请确认是否继续？输入“确认开启”继续: ")

	line, err := in.ReadString('\n')
	if err != nil {
		fmt.Fprintln(out, "读取输入失败，已取消。")
		return false
	}
	if strings.TrimSpace(line) != "确认开启" {
		fmt.Fprintln(out, "已取消开启 BBR。")
		return false
	}
	return true
}
