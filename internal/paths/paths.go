package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	RootDir string

	SingBoxPath string
	ConfigPath  string
	LogPath     string

	OpenRCOutLogPath string
	OpenRCErrLogPath string

	ServiceName string
	ServiceFile string
}

func Discover() (Paths, error) {
	if v := os.Getenv("ALPINE_VLESS_HOME"); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return Paths{}, err
		}
		rootDir := filepath.Clean(abs)
		if rootDir == "" || rootDir == "/" || rootDir == "." {
			return Paths{}, errors.New("ALPINE_VLESS_HOME 非法：禁止为根目录或当前目录")
		}
		return Paths{
			RootDir: rootDir,

			SingBoxPath: filepath.Join(rootDir, "sing-box"),
			ConfigPath:  filepath.Join(rootDir, "config.json"),
			LogPath:     filepath.Join(rootDir, "sing-box.log"),

			OpenRCOutLogPath: filepath.Join(rootDir, "openrc.out.log"),
			OpenRCErrLogPath: filepath.Join(rootDir, "openrc.err.log"),

			ServiceName: "alpine-vless",
			ServiceFile: "/etc/init.d/alpine-vless",
		}, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return Paths{}, err
	}

	exeDir := filepath.Dir(exe)
	if exeDir == "" || exeDir == "/" || exeDir == "." {
		return Paths{}, fmt.Errorf("无法确定可写入的运行目录: %q", exeDir)
	}

	rootDir := filepath.Join(exeDir, "alpine-vless-data")
	return Paths{
		RootDir: rootDir,

		SingBoxPath: filepath.Join(rootDir, "sing-box"),
		ConfigPath:  filepath.Join(rootDir, "config.json"),
		LogPath:     filepath.Join(rootDir, "sing-box.log"),

		OpenRCOutLogPath: filepath.Join(rootDir, "openrc.out.log"),
		OpenRCErrLogPath: filepath.Join(rootDir, "openrc.err.log"),

		ServiceName: "alpine-vless",
		ServiceFile: "/etc/init.d/alpine-vless",
	}, nil
}
