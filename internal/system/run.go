package system

import (
	"context"
	"fmt"
	"os/exec"
)

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v 失败: %w: %s", name, args, err, string(out))
	}
	return nil
}
