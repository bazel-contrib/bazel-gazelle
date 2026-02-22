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

// This file contains integration tests for all of Gazelle. It's meant to test
// common usage patterns and check for errors that are difficult to test in
// unit tests.

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/internal/wspace"
	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/google/go-cmp/cmp"
)

// skipIfWorkspaceVisible skips the test if the WORKSPACE file for the
// repository is visible. This happens in newer Bazel versions when tests
// are run without sandboxing, since temp directories may be inside the
// exec root.
func skipIfWorkspaceVisible(t *testing.T, dir string) {
	if parent, err := wspace.FindRepoRoot(dir); err == nil {
		t.Skipf("WORKSPACE visible in parent %q of tmp %q", parent, dir)
	}
}

func runGazelle(wd string, args []string) error {
	return run(wd, args)
}

// TestHelp checks that help commands do not panic due to nil flag values.
// Verifies #256.
func TestHelp(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"fix", "-h"},
		{"update", "-h"},
		{"update-repos", "-h"},
	} {
		t.Run(args[0], func(t *testing.T) {
			if err := runGazelle(".", args); err == nil {
				t.Errorf("%s: got success, want flag.ErrHelp", args[0])
			} else if err != flag.ErrHelp {
				t.Errorf("%s: got %v, want flag.ErrHelp", args[0], err)
			}
		})
	}
}

func TestNoRepoRootOrWorkspace(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, nil)
	defer cleanup()
	skipIfWorkspaceVisible(t, dir)
	want := "-repo_root not specified"
	if err := runGazelle(dir, nil); err == nil {
		t.Fatalf("got success; want %q", want)
	} else if !strings.Contains(err.Error(), want) {
		t.Fatalf("got %q; want %q", err, want)
	}
}

func TestErrorOutsideWorkspace(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "a/"},
		{Path: "b/"},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()
	skipIfWorkspaceVisible(t, dir)

	cases := []struct {
		name, dir, want string
		args            []string
	}{
		{
			name: "outside workspace",
			dir:  dir,
			args: nil,
			want: "WORKSPACE cannot be found",
		}, {
			name: "outside repo_root",
			dir:  filepath.Join(dir, "a"),
			args: []string{"-repo_root", filepath.Join(dir, "b")},
			want: "not a subdirectory of repo root",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := runGazelle(c.dir, c.args); err == nil {
				t.Fatalf("got success; want %q", c.want)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %q; want %q", err, c.want)
			}
		})
	}
}

func TestBuildFileNameIgnoresBuild(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "BUILD/"},
		{
			Path:    "a/BUILD",
			Content: "!!! parse error",
		},
		{
			Path:    "a.go",
			Content: "package a",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"-go_prefix", "example.com/foo", "-build_file_name", "BUILD.bazel"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "BUILD.bazel")); err != nil {
		t.Errorf("BUILD.bazel not created: %v", err)
	}
}

func TestUpdateReposWithQueryToWorkspace(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "8df59f11fb697743cbb3f26cfb8750395f30471e9eabde0d174c3aebc7a1cd39",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/0.19.1/rules_go-0.19.1.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/0.19.1/rules_go-0.19.1.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "be9296bfd64882e3c08e3283c58fcb461fa6dd3c171764fcc4cf322f60615a9b",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/0.18.1/bazel-gazelle-0.18.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/0.18.1/bazel-gazelle-0.18.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(nogo = "@bazel_gazelle//:nogo")

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "github.com/sirupsen/logrus@v1.3.0"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "8df59f11fb697743cbb3f26cfb8750395f30471e9eabde0d174c3aebc7a1cd39",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/0.19.1/rules_go-0.19.1.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/0.19.1/rules_go-0.19.1.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "be9296bfd64882e3c08e3283c58fcb461fa6dd3c171764fcc4cf322f60615a9b",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/0.18.1/bazel-gazelle-0.18.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/0.18.1/bazel-gazelle-0.18.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(nogo = "@bazel_gazelle//:nogo")

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

go_repository(
    name = "com_github_sirupsen_logrus",
    importpath = "github.com/sirupsen/logrus",
    sum = "h1:hI/7Q+DtNZ2kINb6qt/lS+IyXnHQe9e90POfeewL/ME=",
    version = "v1.3.0",
)

gazelle_dependencies()
`,
		},
	})
}

func TestFixWorkspaceWithoutGazelle(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_repository")

go_repository(
    name = "com_example_repo",
    importpath = "example.com/repo",
    tag = "1.2.3",
)
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"fix", "-go_prefix="}); err == nil {
		t.Error("got success; want error")
	} else if want := "bazel_gazelle is not declared"; !strings.Contains(err.Error(), want) {
		t.Errorf("got error %v; want error containing %q", err, want)
	}
}

func TestImportCollision(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "go.mod",
			Content: `
module example.com/importcases

go 1.13

require (
	github.com/Selvatico/go-mocket v1.0.7
	github.com/selvatico/go-mocket v1.0.7
)
`,
		},
		{
			Path: "go.sum",
			Content: `
github.com/Selvatico/go-mocket v1.0.7/go.mod h1:4gO2v+uQmsL+jzQgLANy3tyEFzaEzHlymVbZ3GP2Oes=
github.com/selvatico/go-mocket v1.0.7/go.mod h1:7bSWzuNieCdUlanCVu3w0ppS0LvDtPAZmKBIlhoTcp8=
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "--from_file=go.mod"}
	errMsg := "imports github.com/Selvatico/go-mocket and github.com/selvatico/go-mocket resolve to the same repository rule name com_github_selvatico_go_mocket"
	if err := runGazelle(dir, args); err == nil {
		t.Fatal("expected error, got nil")
	} else if err.Error() != errMsg {
		t.Errorf("want %s, got %s", errMsg, err.Error())
	}
}

func TestImportCollisionWithReplace(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path:    "WORKSPACE",
			Content: "# gazelle:repo bazel_gazelle",
		},
		{
			Path: "go.mod",
			Content: `
module github.com/linzhp/go_examples/importcases

go 1.13

require (
	github.com/Selvatico/go-mocket v1.0.7
	github.com/selvatico/go-mocket v0.0.0-00010101000000-000000000000
)

replace github.com/selvatico/go-mocket => github.com/Selvatico/go-mocket v1.0.7
`,
		},
		{
			Path: "go.sum",
			Content: `
github.com/Selvatico/go-mocket v1.0.7/go.mod h1:4gO2v+uQmsL+jzQgLANy3tyEFzaEzHlymVbZ3GP2Oes=
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "--from_file=go.mod"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_gazelle//:deps.bzl", "go_repository")

# gazelle:repo bazel_gazelle

go_repository(
    name = "com_github_selvatico_go_mocket",
    importpath = "github.com/selvatico/go-mocket",
    replace = "github.com/Selvatico/go-mocket",
    sum = "h1:sXuFMnMfVL9b/Os8rGXPgbOFbr4HJm8aHsulD/uMTUk=",
    version = "v1.0.7",
)
`,
		},
	})
}

// TestUpdateReposWithGlobalBuildTags is a regresion test for issue #711.
// It also ensures that existings build_tags get merged with requested build_tags.
func TestUpdateReposWithGlobalBuildTags(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_gazelle//:deps.bzl", "go_repository")

# gazelle:repo bazel_gazelle

go_repository(
    name = "com_github_selvatico_go_mocket",
    build_tags = [
        "bar",
    ],
    importpath = "github.com/selvatico/go-mocket",
    replace = "github.com/Selvatico/go-mocket",
    sum = "h1:sXuFMnMfVL9b/Os8rGXPgbOFbr4HJm8aHsulD/uMTUk=",
    version = "v1.0.7",
)
`,
		},
		{
			Path: "go.mod",
			Content: `
module github.com/linzhp/go_examples/importcases

go 1.13

require (
	github.com/Selvatico/go-mocket v1.0.7
	github.com/selvatico/go-mocket v0.0.0-00010101000000-000000000000
)

replace github.com/selvatico/go-mocket => github.com/Selvatico/go-mocket v1.0.7
`,
		},
		{
			Path: "go.sum",
			Content: `
github.com/Selvatico/go-mocket v1.0.7/go.mod h1:4gO2v+uQmsL+jzQgLANy3tyEFzaEzHlymVbZ3GP2Oes=
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "--from_file=go.mod", "--build_tags=bar,foo"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_gazelle//:deps.bzl", "go_repository")

# gazelle:repo bazel_gazelle

go_repository(
    name = "com_github_selvatico_go_mocket",
    build_tags = [
        "bar",
        "foo",
    ],
    importpath = "github.com/selvatico/go-mocket",
    replace = "github.com/Selvatico/go-mocket",
    sum = "h1:sXuFMnMfVL9b/Os8rGXPgbOFbr4HJm8aHsulD/uMTUk=",
    version = "v1.0.7",
)
`,
		},
	})
}

func TestUpdateRepos_LangFilter(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
	})
	defer cleanup()

	args := []string{"update-repos", "-lang=proto", "github.com/sirupsen/logrus@v1.3.0"}
	err := runGazelle(dir, args)
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	if !strings.Contains(err.Error(), "no languages can update repositories") {
		t.Fatalf("unexpected error: %+v", err)
	}
}

func TestUpdateReposOldBoilerplateNewRepo(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "golang.org/x/mod@v0.3.0"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:RM4zey1++hCTbCVQfnWeKs9/IEsaBLA8vTkd0WVtmH4=",
    version = "v0.3.0",
)

gazelle_dependencies()
`,
		},
	})
}

func TestUpdateReposSkipsDirectiveRepo(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

# gazelle:repository go_repository name=org_golang_x_mod importpath=golang.org/x/mod
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "golang.org/x/mod@v0.3.0"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

# gazelle:repository go_repository name=org_golang_x_mod importpath=golang.org/x/mod
`,
		},
	})
}

func TestUpdateReposOldBoilerplateNewMacro(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "-to_macro=deps.bzl%deps", "golang.org/x/mod@v0.3.0"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("//:deps.bzl", "deps")

# gazelle:repository_macro deps.bzl%deps
deps()

gazelle_dependencies()
`,
		},
	})
}

func TestUpdateReposNewBoilerplateNewRepo(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

gazelle_dependencies()
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "golang.org/x/mod@v0.3.0"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:RM4zey1++hCTbCVQfnWeKs9/IEsaBLA8vTkd0WVtmH4=",
    version = "v0.3.0",
)

go_rules_dependencies()

go_register_toolchains()

gazelle_dependencies()
`,
		},
	})
}

func TestUpdateReposNewBoilerplateNewMacro(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

gazelle_dependencies()
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update-repos", "-to_macro=deps.bzl%deps", "golang.org/x/mod@v0.3.0"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2697f6bc7c529ee5e6a2d9799870b9ec9eaeb3ee7d70ed50b87a2c2c97e13d9e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.8/rules_go-v0.23.8.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:deps.bzl", "deps")

# gazelle:repository_macro deps.bzl%deps
deps()

go_rules_dependencies()

go_register_toolchains()

gazelle_dependencies()
`,
		},
	})
}

// Checks that go:embed directives with spaces and quotes are parsed correctly.
// This probably belongs in //language/go:go_test, but we need file names with
// spaces, and Bazel doesn't allow those in runfiles, which that test depends
// on.
func TestQuotedEmbedsrcs(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path:    "BUILD.bazel",
			Content: "# gazelle:prefix example.com/foo",
		},
		{
			Path: "foo.go",
			Content: strings.Join([]string{
				"package foo",
				"import \"embed\"",
				"//go:embed q1.txt q2.txt \"q 3.txt\" `q 4.txt`",
				"var fs embed.FS",
			}, "\n"),
		},
		{
			Path: "q1.txt",
		},
		{
			Path: "q2.txt",
		},
		{
			Path: "q 3.txt",
		},
		{
			Path: "q 4.txt",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"update"}); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{{
		Path: "BUILD.bazel",
		Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix example.com/foo

go_library(
    name = "foo",
    srcs = ["foo.go"],
    embedsrcs = [
        "q 3.txt",
        "q 4.txt",
        "q1.txt",
        "q2.txt",
    ],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
)
`,
	}})
}

// TestUpdateReposDoesNotModifyGoSum verifies that commands executed by
// update-repos do not modify go.sum, particularly 'go mod download' when
// a sum is missing. Verifies #990.
//
// This could also be tested in language/go/update_import_test.go, but that
// test relies on stubs for speed, and it's important to run the real
// go command here.
func TestUpdateReposDoesNotModifyGoSum(t *testing.T) {
	if testing.Short() {
		// Test may download small files over network.
		t.Skip()
	}
	goSumFile := testtools.FileSpec{
		// go.sum only contains the sum for the mod file, not the content.
		// This is common for transitive dependencies not needed by the main module.
		Path:    "go.sum",
		Content: "golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=\n",
	}
	files := []testtools.FileSpec{
		{
			Path:    "WORKSPACE",
			Content: "# gazelle:repo bazel_gazelle",
		},
		{
			Path: "go.mod",
			Content: `
module test

go 1.16

require golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
`,
		},
		goSumFile,
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"update-repos", "-from_file=go.mod"}); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{goSumFile})
}

func TestResolveGoStaticFromGoMod(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path: "go.mod",
			Content: `
module example.com/use

go 1.19

require example.com/dep v1.0.0
`,
		},
		{
			Path: "use.go",
			Content: `
package use

import _ "example.com/dep/pkg"
`,
		},
	})
	defer cleanup()

	args := []string{
		"-go_prefix=example.com/use",
		"-external=static",
		"-go_naming_convention_external=import",
	}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "use",
    srcs = ["use.go"],
    importpath = "example.com/use",
    visibility = ["//visibility:public"],
    deps = ["@com_example_dep//pkg"],
)
`,
		},
	})
}

func TestMigrateSelectFromWorkspaceToBzlmod(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path:    "MODULE.bazel",
			Content: `bazel_dep(name = "rules_go", version = "0.39.1", repo_name = "my_rules_go")`,
		},
		{
			Path: "BUILD",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = [
        "bar.go",
        "foo.go",
        "foo_android.go",
        "foo_android_build_tag.go",
    ],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
    deps = select({
        "@io_bazel_rules_go//go/platform:android": [
            "//outer",
            "//outer/inner",
            "//outer_android_build_tag",
            "//outer_android_suffix",
            "@com_github_jr_hacker_tools//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//outer",
            "//outer/inner",
            "@com_github_jr_hacker_tools//:go_default_library",
        ],
        "//conditions:default": [],
    }),
)
`,
		},
		{
			Path: "foo.go",
			Content: `
// +build linux

package foo

import (
    _ "example.com/foo/outer"
    _ "example.com/foo/outer/inner"
    _ "github.com/jr_hacker/tools"
)
`,
		},
		{
			Path: "foo_android_build_tag.go",
			Content: `
// +build android

package foo

import (
    _ "example.com/foo/outer_android_build_tag"
)
`,
		},
		{
			Path: "foo_android.go",
			Content: `
package foo

import (
    _ "example.com/foo/outer_android_suffix"
)
`,
		},
		{
			Path: "bar.go",
			Content: `// +build linux

package foo
`,
		},
		{Path: "outer/outer.go", Content: "package outer"},
		{Path: "outer_android_build_tag/outer.go", Content: "package outer_android_build_tag"},
		{Path: "outer_android_suffix/outer.go", Content: "package outer_android_suffix"},
		{Path: "outer/inner/inner.go", Content: "package inner"},
	})
	want := `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = [
        "bar.go",
        "foo.go",
        "foo_android.go",
        "foo_android_build_tag.go",
    ],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
    deps = select({
        "@my_rules_go//go/platform:android": [
            "//outer",
            "//outer/inner",
            "//outer_android_build_tag",
            "//outer_android_suffix",
            "@com_github_jr_hacker_tools//:go_default_library",
        ],
        "@my_rules_go//go/platform:linux": [
            "//outer",
            "//outer/inner",
            "@com_github_jr_hacker_tools//:go_default_library",
        ],
        "//conditions:default": [],
    }),
)
`
	defer cleanup()

	if err := runGazelle(dir, []string{"-go_prefix", "example.com/foo"}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "BUILD")); err != nil {
		t.Fatal(err)
	} else if string(got) != want {
		t.Fatalf("got %s ; want %s; diff %s", string(got), want, cmp.Diff(string(got), want))
	}
}

func TestUpdateReposWithBzlmodWithToMacro(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path: "go.mod",
			Content: `
module example.com/foo/v2

go 1.19

require (
	github.com/stretchr/testify v1.8.4
)
`,
		},
	})

	t.Cleanup(cleanup)

	args := []string{
		"update-repos",
		"-from_file=go.mod",
		"-to_macro=go_deps.bzl%my_go_deps",
		"-bzlmod",
	}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	// Confirm that the WORKSPACE is still empty
	want := ""
	if got, err := os.ReadFile(filepath.Join(dir, "WORKSPACE")); err != nil {
		t.Fatal(err)
	} else if string(got) != want {
		t.Fatalf("got %s ; want %s; diff %s", string(got), want, cmp.Diff(string(got), want))
	}

	// Confirm that the macro file was written
	want = `load("@bazel_gazelle//:deps.bzl", "go_repository")

def my_go_deps():
    go_repository(
        name = "com_github_stretchr_testify",
        importpath = "github.com/stretchr/testify",
        sum = "h1:CcVxjf3Q8PM0mHUKJCdn+eZZtm5yQwehR5yeSVQQcUk=",
        version = "v1.8.4",
    )
`
	if got, err := os.ReadFile(filepath.Join(dir, "go_deps.bzl")); err != nil {
		t.Fatal(err)
	} else if string(got) != want {
		t.Fatalf("got %s ; want %s; diff %s", string(got), want, cmp.Diff(string(got), want))
	}
}

func TestUpdateReposWithBzlmodWithoutToMacro(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path: "go.mod",
			Content: `
module example.com/foo/v2

go 1.19

require (
	github.com/stretchr/testify v1.8.4
)
`,
		},
	})

	t.Cleanup(cleanup)

	args := []string{
		"update-repos",
		"-from_file=go.mod",
		"-bzlmod",
	}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	// Confirm that the WORKSPACE is still empty
	want := ""
	if got, err := os.ReadFile(filepath.Join(dir, "WORKSPACE")); err != nil {
		t.Fatal(err)
	} else if string(got) != want {
		t.Fatalf("got %s ; want %s; diff %s", string(got), want, cmp.Diff(string(got), want))
	}
}

func TestCgoFlagsHaveExternalPrefix(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path:    "external/com_example_foo_v2/go.mod",
			Content: "module example.com/foo/v2",
		}, {
			Path: "external/com_example_foo_v2/cgo_static.go",
			Content: `
package duckdb

/*
#cgo LDFLAGS: -lstdc++ -lm -ldl -L${SRCDIR}/deps
*/
import "C"
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	repoRoot := filepath.Join(dir, "external", "com_example_foo_v2")

	args := []string{"update", "-repo_root", repoRoot, "-go_prefix", "example.com/foo/v2", "-go_repository_mode", "-go_repository_module_mode"}
	if err := runGazelle(repoRoot, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, repoRoot, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = ["cgo_static.go"],
    cgo = True,
    clinkopts = ["-lstdc++ -lm -ldl -Lexternal/com_example_foo_v2/deps"],
    importpath = "example.com/foo/v2",
    importpath_aliases = ["example.com/foo"],
    visibility = ["//visibility:public"],
)
`,
		},
	})
}

func TestCgoFlagsHaveDotDotPrefixWithSiblingRepositoryLayout(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path:    "execroot/com_example_foo_v2/go.mod",
			Content: "module example.com/foo/v2",
		}, {
			Path: "execroot/com_example_foo_v2/cgo_static.go",
			Content: `
package duckdb

/*
#cgo LDFLAGS: -lstdc++ -lm -ldl -L${SRCDIR}/deps
*/
import "C"
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	repoRoot := filepath.Join(dir, "execroot", "com_example_foo_v2")

	args := []string{"update", "-repo_root", repoRoot, "-go_prefix", "example.com/foo/v2", "-go_repository_mode", "-go_repository_module_mode"}
	if err := runGazelle(repoRoot, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, repoRoot, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = ["cgo_static.go"],
    cgo = True,
    clinkopts = ["-lstdc++ -lm -ldl -L../com_example_foo_v2/deps"],
    importpath = "example.com/foo/v2",
    importpath_aliases = ["example.com/foo"],
    visibility = ["//visibility:public"],
)
`,
		},
	})
}

// TestEmptyTestdataNoData checks that an empty testdata subdirectory does not
// result in a data attribute being added to go_test rules.
func TestEmptyTestdataNoData(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path: "example_test.go",
			Content: `
package example

import "testing"

func TestExample(t *testing.T) {}
`,
		},
		{Path: "testdata/"},
	})
	defer cleanup()

	if err := runGazelle(dir, []string{"-go_prefix", "example.com/foo"}); err != nil {
		t.Fatal(err)
	}

	buildContent, err := os.ReadFile(filepath.Join(dir, "BUILD.bazel"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the BUILD file doesn't contain a data attribute
	if strings.Contains(string(buildContent), "data = ") {
		t.Errorf("BUILD file should not contain data attribute for empty testdata\n%s", string(buildContent))
	}

	// Verify that a go_test rule was generated
	if !strings.Contains(string(buildContent), "go_test(") {
		t.Errorf("BUILD file should contain a go_test rule\n%s", string(buildContent))
	}
}
