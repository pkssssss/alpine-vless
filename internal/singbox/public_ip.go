package singbox

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func PublicIP(ctx context.Context, httpClient *http.Client) (string, error) {
	ip, err := fetchPublicIP(ctx, httpClient, "https://api.ipify.org")
	if err == nil && ip != "" {
		return ip, nil
	}
	fallbackIP, fallbackErr := fetchPublicIP(ctx, httpClient, "https://api64.ipify.org")
	if fallbackErr == nil && fallbackIP != "" {
		return fallbackIP, nil
	}
	return "", fmt.Errorf("获取公网 IP 失败: 主接口错误=%v; 备用接口错误=%w", err, fallbackErr)
}

func fetchPublicIP(ctx context.Context, httpClient *http.Client, url string) (string, error) {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = "无响应内容"
		}
		return "", fmt.Errorf("获取公网 IP 失败: %s (HTTP %d): %s", url, resp.StatusCode, msg)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(b))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("获取公网 IP 失败: 返回内容不是合法 IP: %q", ip)
	}
	return ip, nil
}
