package main

import (
	"os/exec"
	"runtime"
	"sync"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	goBinaryOnce sync.Once
	goBinaryPath string
	goBinaryErr  error
)

func goBinary() (string, error) {
	goBinaryOnce.Do(func() {
		rlocationPath := "go_sdk/bin/go"
		if runtime.GOOS == "windows" {
			rlocationPath += ".exe"
		}
		path, err := runfiles.Rlocation(rlocationPath)
		if err != nil {
			goBinaryErr = err
			return
		}
		goBinaryPath = path
	})
	return goBinaryPath, goBinaryErr
}

func goCommand(dirPath string, args ...string) (*exec.Cmd, error) {
	bin, err := goBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dirPath
	return cmd, nil
}
