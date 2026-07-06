package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type GoModDownloadResult struct {
	Dir   string
	Sum   string
	Error string
}

func findGoPath() string {
	// Locate the go binary. If GOROOT is set, we'll use that one; otherwise,
	// we'll use PATH.
	goPath := "go"
	if runtime.GOOS == "windows" {
		goPath += ".exe"
	}
	if goroot, ok := os.LookupEnv("GOROOT"); ok {
		goPath = filepath.Join(goroot, "bin", goPath)
	}
	return goPath
}

func runGoModDownload(dl *GoModDownloadResult, dest string, importpath string, version string) error {
	buf := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	cmd := exec.Command(findGoPath(), "mod", "download", "-json", "-modcacherw")
	cmd.Dir = dest

	if version != "" && importpath != "" {
		cmd.Args = append(cmd.Args, importpath+"@"+version)
	}

	cmd.Stdout = buf
	cmd.Stderr = bufErr
	dlErr := cmd.Run()
	if dlErr != nil {
		if _, ok := dlErr.(*exec.ExitError); !ok {
			if bufErr.Len() > 0 {
				return fmt.Errorf("go mod download exec error: %s %q: %s, %w", cmd.Path, strings.Join(cmd.Args, " "), bufErr.String(), dlErr)
			}
			return fmt.Errorf("go mod download exec error: %s %s: %v", cmd.Path, strings.Join(cmd.Args, " "), dlErr)
		}
	}

	// Parse the JSON output.
	if err := json.Unmarshal(buf.Bytes(), &dl); err != nil {
		// A failed download (e.g. a go.sum checksum mismatch) may print to
		// stderr with no JSON on stdout. Surface stderr instead of a JSON error.
		if dlErr != nil && bufErr.Len() > 0 {
			return fmt.Errorf("go mod download error: `%s %s`: %s", cmd.Path, strings.Join(cmd.Args, " "), strings.TrimSpace(bufErr.String()))
		}
		if bufErr.Len() > 0 {
			return fmt.Errorf("go mod download output format: `%s %s`: parsing JSON: %q stderr: %q error: %w", cmd.Path, strings.Join(cmd.Args, " "), buf.String(), bufErr.String(), err)
		}
		return fmt.Errorf("go mod download output format: `%s %s`: parsing JSON: %q error: %w", cmd.Path, strings.Join(cmd.Args, " "), buf.String(), err)
	}
	if dl.Error != "" {
		return Join(errors.New(dl.Error), dlErr)
	}
	if dlErr != nil {
		return dlErr
	}
	return nil
}
