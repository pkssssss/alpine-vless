package singbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var versionLinePattern = regexp.MustCompile(`(?im)\bsing-box\s+version\s+([^\s]+)`)

func BinaryVersion(ctx context.Context, binaryPath string) (string, error) {
	if strings.TrimSpace(binaryPath) == "" {
		return "", errors.New("binaryPath 不能为空")
	}

	tryArgs := [][]string{
		{"version"},
		{"-v"},
		{"--version"},
	}

	var lastErr error
	for _, args := range tryArgs {
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		out, err := cmd.CombinedOutput()

		if v, parseErr := parseVersionOutput(string(out)); parseErr == nil {
			return v, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("执行 %q %v 失败: %w: %s", binaryPath, args, err, strings.TrimSpace(string(out)))
			continue
		}
		lastErr = fmt.Errorf("无法解析版本输出: %s", strings.TrimSpace(string(out)))
	}

	if lastErr == nil {
		lastErr = errors.New("无法读取 sing-box 版本")
	}
	return "", lastErr
}

func parseVersionOutput(out string) (string, error) {
	s := strings.TrimSpace(out)
	if s == "" {
		return "", errors.New("版本输出为空")
	}

	if m := versionLinePattern.FindStringSubmatch(s); len(m) == 2 {
		v := normalizeVersion(m[1])
		if v != "" {
			return v, nil
		}
	}

	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		for i := 0; i+1 < len(fields); i++ {
			if strings.EqualFold(fields[i], "version") {
				v := normalizeVersion(fields[i+1])
				if v != "" {
					return v, nil
				}
			}
		}
	}

	return "", fmt.Errorf("无法从输出解析版本: %q", s)
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}
