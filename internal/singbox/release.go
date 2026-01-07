package singbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

func LatestVersion(ctx context.Context, httpClient *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/SagerNet/sing-box/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "alpine-vless-installer")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", wrapHTTPDoError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return "", errors.New("GitHub API 限流：可设置环境变量 GITHUB_TOKEN（Personal Access Token）或稍后重试")
		}
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = "无响应内容"
		}
		return "", errors.New("获取最新版本失败（GitHub API）：HTTP " + resp.Status + "：" + msg)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	v := strings.TrimSpace(payload.TagName)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return "", errors.New("获取最新版本失败：tag_name 为空")
	}
	return v, nil
}
