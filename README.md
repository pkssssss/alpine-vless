# alpine-vless

单文件 Go 二进制：在 **Alpine Linux + OpenRC** 上自动部署 **最新 sing-box**，固定搭建 **VLESS + Reality**（单节点），并输出可一键导入的 `vless://` URL。

## 特性

- 单节点极简模型：**添加=重生成/覆盖**，**删除=卸载清空**
- 自动安装 OpenRC 服务并设置开机自启
- 尽量不依赖 `apk add`（下载/解压/配置生成均由程序完成）

## 运行要求

- Alpine Linux（OpenRC）
- root 权限
- 可访问 GitHub（拉取 sing-box release）

## 使用

首次运行若未检测到配置，会自动安装并生成配置：

```sh
./alpine-vless
```

菜单：

- 1.添加配置（重生成/覆盖）
- 2.查看配置（输出一键导入 URL）
- 3.删除配置（卸载/清空，需要输入“确认卸载”）

## 目录与服务

- 默认数据目录：`<二进制所在目录>/alpine-vless-data/`
  - `sing-box`、`config.json`、日志文件等
- OpenRC 服务：
  - 服务名：`alpine-vless`
  - 服务文件：`/etc/init.d/alpine-vless`

可通过环境变量指定数据目录：

```sh
export ALPINE_VLESS_HOME="/root/alpine-vless-data"
```

## 常见问题

- GitHub API 限流：设置 `GITHUB_TOKEN` 后重试
- HTTPS 证书错误：通常是系统缺少 CA 证书（可按错误提示安装 `ca-certificates` 并更新证书）

## 构建

本机构建：

```sh
go build -o alpine-vless ./cmd/alpine-vless
```

交叉编译 Linux amd64：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ./dist/alpine-vless-linux-amd64 ./cmd/alpine-vless
```

