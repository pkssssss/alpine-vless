package system

import (
	"bufio"
	"os"
	"strings"
)

func IsAlpine() bool {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "ID=") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "ID="))
		v = strings.Trim(v, `"'`)
		return strings.EqualFold(v, "alpine")
	}
	return false
}

