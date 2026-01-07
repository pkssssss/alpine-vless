package singbox

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkssssss/alpine-vless/internal/system"
)

type InstallSpec struct {
	Version  string
	Arch     string
	DestPath string
}

func Install(ctx context.Context, httpClient *http.Client, spec InstallSpec) error {
	if spec.Version == "" || spec.Arch == "" || spec.DestPath == "" {
		return errors.New("安装参数不完整")
	}

	url := fmt.Sprintf(
		"https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-linux-%s.tar.gz",
		spec.Version, spec.Version, spec.Arch,
	)

	tmp, err := os.CreateTemp("", "sing-box-*.tar.gz")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := downloadToFile(ctx, httpClient, url, tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := extractSingBoxBinary(tmpPath, spec.Version, spec.Arch, spec.DestPath); err != nil {
		return err
	}
	return nil
}

func CheckConfig(ctx context.Context, singBoxPath, configPath string) error {
	return system.Run(ctx, singBoxPath, "check", "-c", configPath)
}

func DetectArch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("未支持的架构: %s", goarch)
	}
}

func downloadToFile(ctx context.Context, httpClient *http.Client, url string, f *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "alpine-vless-installer")

	resp, err := httpClient.Do(req)
	if err != nil {
		return wrapHTTPDoError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("下载失败: %s (HTTP %d)", url, resp.StatusCode)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func extractSingBoxBinary(tarGzPath, version, arch, destPath string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	wantSuffix := fmt.Sprintf("sing-box-%s-linux-%s/sing-box", version, arch)

	destDir := filepath.Dir(destPath)
	if err := system.MkdirAll0755(destDir); err != nil {
		return err
	}

	tmpDest := destPath + ".tmp"
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasSuffix(hdr.Name, wantSuffix) {
			continue
		}

		out, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		if err := os.Rename(tmpDest, destPath); err != nil {
			return err
		}
		return os.Chmod(destPath, 0755)
	}

	return errors.New("在压缩包中未找到 sing-box 可执行文件")
}
