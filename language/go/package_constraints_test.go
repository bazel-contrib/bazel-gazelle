/* Copyright 2026 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package golang

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildConstraintsMatchKnownPlatformIgnoredFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool.go")
	if err := os.WriteFile(path, []byte(`//go:build ignore

package pkg

/*
void noop(void){}
*/
import "C"

func F() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info := goFileInfo(path, dir)
	c, _, _ := testConfig(t)
	if !info.isCgo {
		t.Fatal("goFileInfo: want isCgo")
	}
	if _, included := getPlatformStringsAddFunction(c, info, nil); included {
		t.Error("want ignore-tagged CGO snippet excluded from all known platforms")
	}
}

func TestIgnoreTaggedCgoDoesNotSetLibraryCgoFlag(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "lib.go")
	if err := os.WriteFile(goodPath, []byte(`package pkg

func X() int { return 0 }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	toolPath := filepath.Join(dir, "tool.go")
	if err := os.WriteFile(toolPath, []byte(`//go:build ignore

package pkg

/*
void noop(void){}
*/
import "C"

func F() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	c, _, _ := testConfig(t)
	var tgt goTarget
	tgt.addFile(c, nil, goFileInfo(goodPath, dir))
	tgt.addFile(c, nil, goFileInfo(toolPath, dir))
	if tgt.cgo {
		t.Error("want cgo only from buildable imports of C")
	}
}
