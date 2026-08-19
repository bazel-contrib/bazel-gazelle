package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	rf, err := runfiles.New()
	if err != nil {
		t.Fatal(err)
	}
	bzlPaths, err := fs.Glob(rf, "*/tests/bzlmod/go_deps/*.bzl")
	if err != nil {
		t.Fatal(err)
	}
	if len(bzlPaths) == 0 {
		t.Fatal("no test case .bzl files found in runfiles")
	}
	sort.Strings(bzlPaths)

	for _, rlocation := range bzlPaths {
		name := strings.TrimSuffix(filepath.Base(rlocation), ".bzl")
		t.Run(name, func(t *testing.T) {
			bzlPath, err := runfiles.Rlocation(rlocation)
			if err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			outBzl := filepath.Join(dir, filepath.Base(bzlPath))

			if err := convertBzlToDir(bzlPath, dir, true, "/unused"); err != nil {
				t.Fatalf("convertBzlToDir: %v", err)
			}
			if err := convertDirToBzl(dir, outBzl); err != nil {
				t.Fatalf("convertDirToBzl: %v", err)
			}

			orig, err := os.ReadFile(bzlPath)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(outBzl)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(orig, got); diff != "" {
				t.Fatalf("round trip changed test (-want, +got): %s", diff)
			}
		})
	}
}
