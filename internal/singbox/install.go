package singbox

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkssssss/alpine-vless/internal/system"
)

type InstallSpec struct {
	Version  string
	Arch     string
	DestPath string
	Progress io.Writer
}

func Install(ctx context.Context, httpClient *http.Client, spec InstallSpec) error {
	if spec.Version == "" || spec.Arch == "" || spec.DestPath == "" {
		return errors.New("安装参数不完整")
	}

	assetName := fmt.Sprintf("sing-box-%s-linux-%s.tar.gz", spec.Version, spec.Arch)
	url := fmt.Sprintf(
		"https://github.com/SagerNet/sing-box/releases/download/v%s/%s",
		spec.Version, assetName,
	)

	tmp, err := os.CreateTemp("", "sing-box-*.tar.gz")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if spec.Progress != nil {
		fmt.Fprintln(spec.Progress, "正在获取发布校验信息...")
	}
	expectedSHA256, err := releaseAssetSHA256(ctx, httpClient, spec.Version, assetName)
	if err != nil {
		return err
	}
	if spec.Progress != nil {
		fmt.Fprintf(spec.Progress, "校验值: %s...\n", expectedSHA256[:12])
	}

	if err := downloadToFile(ctx, httpClient, url, tmp, spec.Progress); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if spec.Progress != nil {
		fmt.Fprintln(spec.Progress, "正在校验下载文件完整性...")
	}
	if err := verifyFileSHA256(tmpPath, expectedSHA256); err != nil {
		return err
	}
	if spec.Progress != nil {
		fmt.Fprintln(spec.Progress, "完整性校验通过。")
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

func downloadToFile(ctx context.Context, httpClient *http.Client, url string, f *os.File, progress io.Writer) error {
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

	buf := make([]byte, 32*1024)
	var written int64
	lastReport := time.Now()
	lastPercent := int64(-1)

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)

			if progress != nil {
				contentLength := resp.ContentLength
				if contentLength > 0 {
					percent := written * 100 / contentLength
					if percent >= lastPercent+5 || time.Since(lastReport) >= 2*time.Second {
						fmt.Fprintf(progress, "下载进度: %3d%% (%s / %s)\n", percent, formatBytes(written), formatBytes(contentLength))
						lastPercent = percent
						lastReport = time.Now()
					}
				} else if time.Since(lastReport) >= 2*time.Second {
					fmt.Fprintf(progress, "下载进度: %s\n", formatBytes(written))
					lastReport = time.Now()
				}
			}
		}

		if errors.Is(rErr, io.EOF) {
			break
		}
		if rErr != nil {
			return rErr
		}
	}

	if progress != nil {
		if resp.ContentLength > 0 {
			fmt.Fprintf(progress, "下载进度: 100%% (%s / %s)\n", formatBytes(written), formatBytes(resp.ContentLength))
		} else {
			fmt.Fprintf(progress, "下载完成: %s\n", formatBytes(written))
		}
	}
	return nil
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func releaseAssetSHA256(ctx context.Context, httpClient *http.Client, version, assetName string) (string, error) {
	assets, err := ReleaseAssetsByTag(ctx, httpClient, version)
	if err != nil {
		return "", err
	}
	if sha, ok := assetDigestSHA256(assets, assetName); ok {
		return sha, nil
	}
	checksumURL, err := findChecksumAssetURL(assets)
	if err != nil {
		return "", err
	}
	checksumText, err := fetchText(ctx, httpClient, checksumURL)
	if err != nil {
		return "", err
	}
	sha, err := parseSHA256FromChecksums(checksumText, assetName)
	if err != nil {
		return "", err
	}
	return sha, nil
}

func assetDigestSHA256(assets []ReleaseAsset, assetName string) (string, bool) {
	for _, a := range assets {
		if strings.TrimSpace(a.Name) != assetName {
			continue
		}
		sha, ok := parseAssetDigestSHA256(a.Digest)
		if ok {
			return sha, true
		}
	}
	return "", false
}

func parseAssetDigestSHA256(digest string) (string, bool) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return "", false
	}

	if strings.Contains(digest, ":") {
		parts := strings.SplitN(digest, ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "sha256") {
			return "", false
		}
		digest = strings.TrimSpace(parts[1])
	}

	digest = strings.ToLower(digest)
	if !isSHA256Hex(digest) {
		return "", false
	}
	return digest, true
}

func findChecksumAssetURL(assets []ReleaseAsset) (string, error) {
	var fallback string
	for _, a := range assets {
		name := strings.ToLower(strings.TrimSpace(a.Name))
		if name == "" || strings.TrimSpace(a.BrowserDownloadURL) == "" {
			continue
		}
		if strings.Contains(name, "checksum") && strings.HasSuffix(name, ".txt") {
			return a.BrowserDownloadURL, nil
		}
		if strings.Contains(name, "sha256") || strings.Contains(name, "checksum") {
			fallback = a.BrowserDownloadURL
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", errors.New("未找到 release 校验文件（checksum/sha256）")
}

func fetchText(ctx context.Context, httpClient *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "alpine-vless-installer")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", wrapHTTPDoError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("下载失败: %s (HTTP %d)", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseSHA256FromChecksums(content, assetName string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if sha, ok := parseChecksumLine(line, assetName); ok {
			return sha, nil
		}
	}
	return "", fmt.Errorf("在校验文件中未找到目标资产的 SHA256: %s", assetName)
}

func parseChecksumLine(line, assetName string) (string, bool) {
	upper := strings.ToUpper(line)
	if strings.HasPrefix(upper, "SHA256 (") {
		closing := strings.Index(line, ")")
		eq := strings.LastIndex(line, "=")
		if closing > 7 && eq > closing {
			name := strings.TrimSpace(line[len("SHA256 ("):closing])
			sha := strings.ToLower(strings.TrimSpace(line[eq+1:]))
			if name == assetName && isSHA256Hex(sha) {
				return sha, true
			}
		}
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", false
	}
	sha := strings.ToLower(fields[0])
	if !isSHA256Hex(sha) {
		return "", false
	}
	name := strings.TrimPrefix(fields[len(fields)-1], "*")
	if name != assetName {
		return "", false
	}
	return sha, true
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, ch := range s {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			return false
		}
	}
	return true
}

func verifyFileSHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if !isSHA256Hex(expected) {
		return fmt.Errorf("非法 SHA256 值: %q", expected)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("下载文件 SHA256 校验失败: expected=%s actual=%s", expected, actual)
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
