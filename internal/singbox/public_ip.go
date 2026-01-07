package singbox

import (
	"context"
	"io"
	"net/http"
	"strings"
)

func PublicIP(ctx context.Context, httpClient *http.Client) (string, error) {
	ip, err := fetchText(ctx, httpClient, "https://api.ipify.org")
	if err == nil && ip != "" {
		return ip, nil
	}
	return fetchText(ctx, httpClient, "https://api64.ipify.org")
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

	b, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
