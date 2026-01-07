package system

import (
	"os"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func MkdirAll0700(path string) error {
	return os.MkdirAll(path, 0700)
}

func MkdirAll0755(path string) error {
	return os.MkdirAll(path, 0755)
}

func RemoveAll(path string) error {
	return os.RemoveAll(path)
}

