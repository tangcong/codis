package utils

import (
	"os"
)

func IsDirExist(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	} else {
		return false, nil
	}
}

func MkDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return nil
}
