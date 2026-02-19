package singbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const githubReleaseAPIBase = "https://api.github.com/repos/SagerNet/sing-box"

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func LatestVersion(ctx context.Context, httpClient *http.Client) (string, error) {
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := fetchGitHubReleaseJSON(ctx, httpClient, githubReleaseAPIBase+"/releases/latest", &payload); err != nil {
		return "", err
	}

	v := strings.TrimSpace(payload.TagName)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return "", errors.New("获取最新版本失败：tag_name 为空")
	}
	return v, nil
}

func ReleaseAssetsByTag(ctx context.Context, httpClient *http.Client, version string) ([]ReleaseAsset, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if v == "" {
		return nil, errors.New("version 不能为空")
	}

	var payload struct {
		Assets []ReleaseAsset `json:"assets"`
	}
	url := fmt.Sprintf("%s/releases/tags/v%s", githubReleaseAPIBase, v)
	if err := fetchGitHubReleaseJSON(ctx, httpClient, url, &payload); err != nil {
		return nil, err
	}
	return payload.Assets, nil
}

func fetchGitHubReleaseJSON(ctx context.Context, httpClient *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "alpine-vless-installer")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return wrapHTTPDoError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return errors.New("GitHub API 限流：可设置环境变量 GITHUB_TOKEN（Personal Access Token）或稍后重试")
		}
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = "无响应内容"
		}
		return fmt.Errorf("GitHub API 请求失败（%s）：HTTP %s：%s", url, resp.Status, msg)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
