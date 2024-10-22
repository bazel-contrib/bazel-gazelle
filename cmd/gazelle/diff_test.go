/* Copyright 2017 The Bazel Authors. All rights reserved.

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

package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
)

func TestDiffExisting(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path: "BUILD.bazel",
			Content: `
# gazelle:prefix example.com/hello
`,
		},
		{
			Path:    "hello.go",
			Content: `package hello`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	wantError := "encountered changes while running diff"
	if err := runGazelle(dir, []string{"-mode=diff", "-patch=p"}); err.Error() != wantError {
		t.Fatalf("got %q; want %q", err, wantError)
	}

	want := append(files, testtools.FileSpec{
		Path: "p",
		Content: `
--- BUILD.bazel	1970-01-01 00:00:00.000000001 +0000
+++ BUILD.bazel	1970-01-01 00:00:00.000000001 +0000
@@ -1,2 +1,10 @@
+load("@io_bazel_rules_go//go:def.bzl", "go_library")
 
 # gazelle:prefix example.com/hello
+
+go_library(
+    name = "hello",
+    srcs = ["hello.go"],
+    importpath = "example.com/hello",
+    visibility = ["//visibility:public"],
+)
`,
	})
	testtools.CheckFiles(t, dir, want)
}

func TestDiffNew(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path:    "hello.go",
			Content: `package hello`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	wantError := "encountered changes while running diff"
	if err := runGazelle(dir, []string{"-go_prefix=example.com/hello", "-mode=diff", "-patch=p"}); err.Error() != wantError {
		t.Fatalf("got %q; want %q", err, wantError)
	}

	want := append(files, testtools.FileSpec{
		Path: "p",
		Content: `
--- /dev/null	1970-01-01 00:00:00.000000001 +0000
+++ BUILD.bazel	1970-01-01 00:00:00.000000001 +0000
@@ -0,0 +1,8 @@
+load("@io_bazel_rules_go//go:def.bzl", "go_library")
+
+go_library(
+    name = "hello",
+    srcs = ["hello.go"],
+    importpath = "example.com/hello",
+    visibility = ["//visibility:public"],
+)
`,
	})
	testtools.CheckFiles(t, dir, want)
}

func TestDiffMissingAndNoChange(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"-go_prefix=example.com/hello", "-mode=diff", "-patch=p"}); err != nil {
		t.Error("Expected no diff, but got a diff.")
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{{Path: "p"}})
}

func TestDiffEmptyAndNoChange(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "BUILD.bazel"},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"-go_prefix=example.com/hello", "-mode=diff", "-patch=p"}); err != nil {
		t.Error("Expected no diff, but got a diff.")
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{{Path: "p"}})
}

func TestDiffReadWriteDir(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path:    "repo/hello.go",
			Content: "package hello",
		}, {
			Path:    "read/BUILD.bazel",
			Content: "# gazelle:prefix example.com/hello",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{
		"-repo_root=repo",
		"-mode=diff",
		"-patch=p",
		"-experimental_read_build_files_dir=read",
		"-experimental_write_build_files_dir=write",
		"repo",
	}

	wantError := "encountered changes while running diff"
	if err := runGazelle(dir, args); err.Error() != wantError {
		t.Fatalf("got %q; want %q", err, wantError)
	}

	wantPatch := fmt.Sprintf(`
--- %s	1970-01-01 00:00:00.000000001 +0000
+++ %s	1970-01-01 00:00:00.000000001 +0000
@@ -1 +1,10 @@
+load("@io_bazel_rules_go//go:def.bzl", "go_library")
+
 # gazelle:prefix example.com/hello
+
+go_library(
+    name = "hello",
+    srcs = ["hello.go"],
+    importpath = "example.com/hello",
+    visibility = ["//visibility:public"],
+)
`,
		filepath.Join(dir, "read", "BUILD.bazel"),
		filepath.Join(dir, "write", "BUILD.bazel"))
	want := append(files, testtools.FileSpec{Path: "p", Content: wantPatch})
	testtools.CheckFiles(t, dir, want)
}
