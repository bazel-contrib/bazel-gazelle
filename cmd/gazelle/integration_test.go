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

// TestNoIndexNoRecurse checks that gazelle behaves correctly with the flags
// -r=false -index=false. Gazelle should not generate build files in
// subdirectories and should not resolve dependencies to local libraries.
func TestNoIndexNoRecurse(t *testing.T) {
	barBuildFile := testtools.FileSpec{
		Path:    "foo/bar/BUILD.bazel",
		Content: "# this should not be updated because -r=false",
	}
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{
			Path: "foo/foo.go",
			Content: `package foo

import (
	_ "example.com/dep/baz"
)
`,
		},
		barBuildFile,
		{
			Path:    "foo/bar/bar.go",
			Content: "package bar",
		},
		{
			Path: "third_party/baz/BUILD.bazel",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

# this should be ignored because -index=false
go_library(
    name = "baz",
    srcs = ["baz.go"],
    importpath = "example.com/dep/baz",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path:    "third_party/baz/baz.go",
			Content: "package baz",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{
		"-go_prefix=example.com/repo",
		"-external=vendored",
		"-r=false",
		"-index=false",
		"foo",
	}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "foo/BUILD.bazel",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/repo/foo",
    visibility = ["//visibility:public"],
    deps = ["//vendor/example.com/dep/baz"],
)
`,
		},
		barBuildFile,
	})
}

// TestNoIndexRecurse checks that gazelle behaves correctly with the flags
// -r=true -index=false. Gazelle should generate build files in directories
// and subdirectories, but should not resolve dependencies to local libraries.
func TestNoIndexRecurse(t *testing.T) {
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"}, {
			Path: "foo/foo.go",
			Content: `package foo

import (
	_ "example.com/dep/baz"
)
`,
		}, {
			Path:    "foo/bar/bar.go",
			Content: "package bar",
		}, {
			Path: "third_party/baz/BUILD.bazel",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

# this should be ignored because -index=false
go_library(
    name = "baz",
    srcs = ["baz.go"],
    importpath = "example.com/dep/baz",
    visibility = ["//visibility:public"],
)
`,
		}, {
			Path:    "third_party/baz/baz.go",
			Content: "package baz",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{
		"-go_prefix=example.com/repo",
		"-external=vendored",
		"-r=true",
		"-index=false",
		"foo",
	}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "foo/BUILD.bazel",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/repo/foo",
    visibility = ["//visibility:public"],
    deps = ["//vendor/example.com/dep/baz"],
)
`,
		}, {
			Path: "foo/bar/BUILD.bazel",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bar",
    srcs = ["bar.go"],
    importpath = "example.com/repo/foo/bar",
    visibility = ["//visibility:public"],
)
`,
		},
	})
}

// TestSubdirectoryPrefixExternal checks that directives set in subdirectories
// may be used in dependency resolution. Verifies #412.
func TestSubdirectoryPrefixExternal(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		}, {
			Path:    "BUILD.bazel",
			Content: "# gazelle:prefix",
		}, {
			Path:    "sub/BUILD.bazel",
			Content: "# gazelle:prefix example.com/sub",
		}, {
			Path: "sub/sub.go",
			Content: `
package sub

import (
	_ "example.com/sub/missing"
	_ "example.com/external"
)
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"-external=vendored", "-index=false"}); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "sub/BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix example.com/sub

go_library(
    name = "sub",
    srcs = ["sub.go"],
    importpath = "example.com/sub",
    visibility = ["//visibility:public"],
    deps = [
        "//sub/missing",
        "//vendor/example.com/external",
    ],
)
`,
		},
	})
}

func TestGoGrpcProtoFlag(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		}, {
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")

go_proto_library(
    name = "foo_go_proto",
    importpath = "example.com/repo/foo",
    proto = ":foo_proto",
    visibility = ["//visibility:public"],
)

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    embed = [":foo_go_proto"],
    importpath = "example.com/repo/foo",
    visibility = ["//visibility:public"],
)
`,
		}, {
			Path: "foo.proto",
			Content: `
syntax = "proto3";

option go_package = "example.com/repo/foo";

message Bar {};
`,
		}, {
			Path: "service/BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_proto_library(
    name = "service_go_proto",
    compilers = [
		"@io_bazel_rules_go//proto:go_proto",
		"@io_bazel_rules_go//proto:go_grpc_v2",
	],
    importpath = "example.com/repo/service",
    proto = ":service_proto",
    visibility = ["//visibility:public"],
)

proto_library(
    name = "service_proto",
    srcs = ["service.proto"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    embed = [":service_go_proto"],
    importpath = "example.com/repo/service",
    visibility = ["//visibility:public"],
)
`,
		}, {
			Path: "service/service.proto",
			Content: `
syntax = "proto3";

option go_package = "example.com/repo/service";

service TestService {}
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update", "-go_grpc_compiler", "//foo", "-go_proto_compiler", "//bar"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

go_proto_library(
    name = "foo_go_proto",
    compilers = ["//bar"],
    importpath = "example.com/repo/foo",
    proto = ":foo_proto",
    visibility = ["//visibility:public"],
)

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    embed = [":foo_go_proto"],
    importpath = "example.com/repo/foo",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "service/BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

go_proto_library(
    name = "service_go_proto",
    compilers = ["//foo"],
    importpath = "example.com/repo/service",
    proto = ":service_proto",
    visibility = ["//visibility:public"],
)

proto_library(
    name = "service_proto",
    srcs = ["service.proto"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    embed = [":service_go_proto"],
    importpath = "example.com/repo/service",
    visibility = ["//visibility:public"],
)
`,
		},
	})
}

// TestMapKind tests the gazelle:map_kind directive.
// Verifies #448
func TestMapKind(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "BUILD.bazel",
			Content: `
# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
`,
		},
		{
			Path:    "root_lib.go",
			Content: `package mapkind`,
		},
		{
			Path:    "enabled/BUILD.bazel",
			Content: "# gazelle:map_kind go_library my_library //tools/go:def.bzl",
		},
		{
			Path:    "enabled/enabled_lib.go",
			Content: `package enabled`,
		},
		{
			Path: "enabled/inherited/BUILD.bazel",
		},
		{
			Path:    "enabled/inherited/inherited_lib.go",
			Content: `package inherited`,
		},
		{
			Path: "enabled/existing_rules/mapped/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "my_library")

# An existing rule with a mapped type is updated
my_library(
    name = "go_default_library",
    srcs = ["deleted_file.go", "mapped_lib.go"],
    importpath = "example.com/mapkind/enabled/existing_rules/mapped",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path:    "enabled/existing_rules/mapped/mapped_lib.go",
			Content: `package mapped`,
		},
		{
			Path:    "enabled/existing_rules/mapped/mapped_lib2.go",
			Content: `package mapped`,
		},
		{
			Path: "enabled/existing_rules/unmapped/BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# An existing rule with an unmapped type is updated
go_library(
    name = "go_default_library",
    srcs = ["unmapped_lib.go"],
    importpath = "example.com/mapkind/enabled/existing_rules/unmapped",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path:    "enabled/existing_rules/unmapped/unmapped_lib.go",
			Content: `package unmapped`,
		},
		{
			Path: "enabled/existing_rules/clean/BUILD.bazel",
			Content: `load("//tools/go:def.bzl", "my_go_library")

# gazelle:go_naming_convention import
# gazelle:map_kind go_library my_go_library //tools/go:def.bzl

my_go_library(
    name = "clean",
    srcs = [
        "foo.go",
        "bar.go",
        "buz.go",
    ],
    importpath = "example.com/mapkind/enabled/existing_rules/clean",
    visibility = ["//visibility:private"],
)
`,
		},
		{
			Path: "enabled/existing_rules/clean_binary/BUILD.bazel",
			Content: `load("//tools/go:def.bzl", "my_go_binary", "my_go_library")

# gazelle:go_naming_convention import
# gazelle:map_kind go_binary my_go_binary //tools/go:def.bzl
# gazelle:map_kind go_library my_go_library //tools/go:def.bzl

my_go_library(
    name = "clean_binary_lib",
    srcs = ["main.go"],
    importpath = "example.com/mapkind/enabled/existing_rules/clean_binary",
    visibility = ["//visibility:private"],
)

my_go_binary(
    name = "clean_binary",
    embed = [":clean_binary_lib"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path:    "enabled/existing_rules/nobuild/nobuild_lib.go",
			Content: `package nobuild`,
		},
		{
			Path:    "enabled/overridden/BUILD.bazel",
			Content: "# gazelle:map_kind go_library overridden_library //tools/overridden:def.bzl",
		},
		{
			Path:    "enabled/overridden/overridden_lib.go",
			Content: `package overridden`,
		},
		{
			Path: "disabled/BUILD.bazel",
		},
		{
			Path:    "disabled/disabled_lib.go",
			Content: `package disabled`,
		},
		{
			Path: "enabled/multiple_mappings/BUILD.bazel",
			Content: `
# gazelle:map_kind go_binary go_binary //tools/go:def.bzl
# gazelle:map_kind go_library go_library //tools/go:def.bzl
`,
		},
		{
			Path: "enabled/multiple_mappings/multiple_mappings.go",
			Content: `
package main

func main() {}`,
		},
		{
			Path: "depend_on_mapped_kind/lib.go",
			Content: `package depend_on_mapped_kind
import (
	_ "example.com/mapkind/disabled"
	_ "example.com/mapkind/enabled"
	_ "example.com/mapkind/enabled/existing_rules/mapped"
	_ "example.com/mapkind/enabled/existing_rules/unmapped"
	_ "example.com/mapkind/enabled/overridden"
)`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"-external=vendored"}); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library

go_library(
    name = "go_default_library",
    srcs = ["root_lib.go"],
    importpath = "example.com/mapkind",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "my_library")

# gazelle:map_kind go_library my_library //tools/go:def.bzl

my_library(
    name = "go_default_library",
    srcs = ["enabled_lib.go"],
    importpath = "example.com/mapkind/enabled",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/inherited/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "my_library")

my_library(
    name = "go_default_library",
    srcs = ["inherited_lib.go"],
    importpath = "example.com/mapkind/enabled/inherited",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/overridden/BUILD.bazel",
			Content: `
load("//tools/overridden:def.bzl", "overridden_library")

# gazelle:map_kind go_library overridden_library //tools/overridden:def.bzl

overridden_library(
    name = "go_default_library",
    srcs = ["overridden_lib.go"],
    importpath = "example.com/mapkind/enabled/overridden",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "disabled/BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["disabled_lib.go"],
    importpath = "example.com/mapkind/disabled",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/existing_rules/mapped/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "my_library")

# An existing rule with a mapped type is updated
my_library(
    name = "go_default_library",
    srcs = [
        "mapped_lib.go",
        "mapped_lib2.go",
    ],
    importpath = "example.com/mapkind/enabled/existing_rules/mapped",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/existing_rules/unmapped/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "my_library")

# An existing rule with an unmapped type is updated
my_library(
    name = "go_default_library",
    srcs = ["unmapped_lib.go"],
    importpath = "example.com/mapkind/enabled/existing_rules/unmapped",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/existing_rules/clean/BUILD.bazel",
			Content: `
# gazelle:go_naming_convention import
# gazelle:map_kind go_library my_go_library //tools/go:def.bzl
`,
		},
		{
			Path: "enabled/existing_rules/clean_binary/BUILD.bazel",
			Content: `
# gazelle:go_naming_convention import
# gazelle:map_kind go_binary my_go_binary //tools/go:def.bzl
# gazelle:map_kind go_library my_go_library //tools/go:def.bzl
`,
		},
		{
			Path: "enabled/existing_rules/nobuild/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "my_library")

my_library(
    name = "go_default_library",
    srcs = ["nobuild_lib.go"],
    importpath = "example.com/mapkind/enabled/existing_rules/nobuild",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "enabled/multiple_mappings/BUILD.bazel",
			Content: `
load("//tools/go:def.bzl", "go_binary", "go_library")

# gazelle:map_kind go_binary go_binary //tools/go:def.bzl
# gazelle:map_kind go_library go_library //tools/go:def.bzl

go_library(
    name = "go_default_library",
    srcs = ["multiple_mappings.go"],
    importpath = "example.com/mapkind/enabled/multiple_mappings",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "multiple_mappings",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "depend_on_mapped_kind/BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["lib.go"],
    importpath = "example.com/mapkind/depend_on_mapped_kind",
    visibility = ["//visibility:public"],
    deps = [
        "//disabled:go_default_library",
        "//enabled:go_default_library",
        "//enabled/existing_rules/mapped:go_default_library",
        "//enabled/existing_rules/unmapped:go_default_library",
        "//enabled/overridden:go_default_library",
    ],
)
`,
		},
	})
}

func TestAliasKind(t *testing.T) {
	for name, tc := range map[string]struct {
		before []testtools.FileSpec
		after  []testtools.FileSpec
		index  bool
	}{
		"existing aliased kind with matching name has deps updated": {
			index: false,
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:alias_kind my_custom_macro go_library
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path: "file.go",
					Content: `
package aliaskind

import (
	_ "example.com/aliaskind/foo"
	_ "github.com/external"
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "BUILD.bazel",
					Content: `
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:alias_kind my_custom_macro go_library
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
    deps = [
        "//foo:go_default_library",
        "//vendor/github.com/external:go_default_library",
    ],
)
`,
				},
			},
		},
		"existing aliased kind with different name has deps updated": {
			index: false,
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:alias_kind my_custom_macro go_library
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "custom_lib",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path: "file.go",
					Content: `
package aliaskind

import (
	_ "example.com/aliaskind/foo"
	_ "github.com/external"
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "BUILD.bazel",
					Content: `
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:alias_kind my_custom_macro go_library
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "custom_lib",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
    deps = [
        "//foo:go_default_library",
        "//vendor/github.com/external:go_default_library",
    ],
)
`,
				},
			},
		},
		"existing aliased kind is indexed for deps": {
			index: true,
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:alias_kind my_custom_macro go_library

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
)
			`,
				},
				{
					Path: "file.go",
					Content: `
package aliaskind

import (
	_ "example.com/aliaskind/foo"
	_ "github.com/external"
)
			`,
				},
				{
					Path: "foo/BUILD.bazel",
					Content: `
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/aliaskind/foo",
    visibility = ["//visibility:public"],
)
			`,
				},
				{
					Path:    "foo/foo.go",
					Content: "package foo",
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "BUILD.bazel",
					Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:alias_kind my_custom_macro go_library

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
    deps = [
        "//foo:go_default_library",
        "//vendor/github.com/external:go_default_library",
    ],
)
			`,
				},
			},
		},
		"alias_kind around a mapped_kind": {
			index: false,
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library my_library //tools:go:def.bzl
# gazelle:alias_kind my_custom_macro my_library
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path: "file.go",
					Content: `
package aliaskind

import (
	_ "example.com/aliaskind/foo"
	_ "github.com/external"
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "BUILD.bazel",
					Content: `
# gazelle:prefix example.com/aliaskind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library my_library //tools:go:def.bzl
# gazelle:alias_kind my_custom_macro my_library
load("//custom:def.bzl", "my_custom_macro")

my_custom_macro(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/aliaskind",
    visibility = ["//visibility:public"],
    deps = [
        "//foo:go_default_library",
        "//vendor/github.com/external:go_default_library",
    ],
)
`,
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir, cleanup := testtools.CreateFiles(t, tc.before)
			t.Cleanup(cleanup)
			args := []string{"-external=vendored"}
			if !tc.index {
				args = append(args, "-index=false")
			}
			if err := runGazelle(dir, args); err != nil {
				t.Fatal(err)
			}
			testtools.CheckFiles(t, dir, tc.after)
		})
	}
}

func TestMapKindEdgeCases(t *testing.T) {
	for name, tc := range map[string]struct {
		before []testtools.FileSpec
		after  []testtools.FileSpec
	}{
		"new generated rule applies map_kind": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"existing generated rule with non-renaming mapping applied applies map_kind": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
`,
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"existing generated rule without non-renaming mapping applied applies map_kind": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
`,
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"existing generated rule without renaming mapping applied applies map_kind": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library custom_go_library //custom:def.bzl
`,
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "custom_go_library")

custom_go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"existing generated rule with renaming mapping applied preserves map_kind": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library custom_go_library //custom:def.bzl
`,
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "custom_go_library")

custom_go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "custom_go_library")

custom_go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"unrelated non-generated non-map_kind'd rule of same kind applies map_kind if other rule is generated": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
		`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "custom_lib",
    srcs = ["custom_lib.go"],
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "custom_lib",
    srcs = ["custom_lib.go"],
)

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"unrelated non-generated non-renaming map_kind'd rule of same kind keeps map_kind if other rule is generated": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
		`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "custom_lib",
    srcs = ["custom_lib.go"],
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library")

go_library(
    name = "custom_lib",
    srcs = ["custom_lib.go"],
)

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"unrelated non-generated non-renaming map_kind'd rule keeps map_kind if other generated rule is newly generated": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
# gazelle:map_kind go_test go_test //custom:def.bzl
		`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_test")

go_test(
    name = "custom_test",
    srcs = ["custom_test.java"],
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library", "go_test")

go_test(
    name = "custom_test",
    srcs = ["custom_test.java"],
)

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"unrelated non-generated non-renaming map_kind'd rule keeps map_kind if other generated rule already existed": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library go_library //custom:def.bzl
# gazelle:map_kind go_test go_test //custom:def.bzl
`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library", "go_test")

go_test(
    name = "custom_test",
    srcs = ["custom_test.java"],
)

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//custom:def.bzl", "go_library", "go_test")

go_test(
    name = "custom_test",
    srcs = ["custom_test.java"],
)

go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
		"transitive remappings are applied": {
			before: []testtools.FileSpec{
				{
					Path: "WORKSPACE",
				},
				{
					Path: "BUILD.bazel",
					Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library custom_go_library //custom:def.bzl
		`,
				},
				{
					Path:    "dir/file.go",
					Content: "package dir",
				},
				{
					Path: "dir/BUILD.bazel",
					Content: `# gazelle:map_kind custom_go_library other_custom_go_library //another/custom:def.bzl
`,
				},
			},
			after: []testtools.FileSpec{
				{
					Path: "dir/BUILD.bazel",
					Content: `load("//another/custom:def.bzl", "other_custom_go_library")

# gazelle:map_kind custom_go_library other_custom_go_library //another/custom:def.bzl

other_custom_go_library(
    name = "go_default_library",
    srcs = ["file.go"],
    importpath = "example.com/mapkind/dir",
    visibility = ["//visibility:public"],
)
`,
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir, cleanup := testtools.CreateFiles(t, tc.before)
			t.Cleanup(cleanup)
			if err := runGazelle(dir, []string{"-external=vendored"}); err != nil {
				t.Fatal(err)
			}
			testtools.CheckFiles(t, dir, tc.after)
		})
	}
}

func TestMapKindLoop(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "BUILD.bazel",
			Content: `# gazelle:prefix example.com/mapkind
# gazelle:go_naming_convention go_default_library
# gazelle:map_kind go_library custom_go_library //custom:def.bzl
		`,
		},
		{
			Path:    "dir/file.go",
			Content: "package dir",
		},
		{
			Path: "dir/BUILD.bazel",
			Content: `# gazelle:map_kind custom_go_library go_library @io_bazel_rules_go//go:def.bzl
`,
		},
	})
	t.Cleanup(cleanup)
	err := runGazelle(dir, []string{"-external=vendored"})
	if err == nil {
		t.Fatal("Expected error running gazelle with map_kind loop")
	}
	msg := err.Error()
	if !strings.Contains(msg, "looking up mapped kind: found loop of map_kind replacements: go_library -> custom_go_library -> go_library") {
		t.Fatalf("Expected error to contain useful descriptors but was %q", msg)
	}
}

// TestMapKindEmbeddedResolve tests the gazelle:map_kind properly resolves
// dependencies for embedded rules (see #1162).
func TestMapKindEmbeddedResolve(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		}, {
			Path: "BUILD.bazel",
			Content: `
# gazelle:prefix example.com/mapkind
# gazelle:map_kind go_library my_go_library //:my.bzl
`,
		}, {
			Path: "a/a.proto",
			Content: `
syntax = "proto3";

package test;
option go_package = "example.com/mapkind/a";
`,
		}, {
			Path: "b/b.proto",
			Content: `
syntax = "proto3";

package test;
option go_package = "example.com/mapkind/b";

import "a/a.proto";
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"-external=vendored"}); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("//:my.bzl", "my_go_library")

proto_library(
    name = "a_proto",
    srcs = ["a.proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "a_go_proto",
    importpath = "example.com/mapkind/a",
    proto = ":a_proto",
    visibility = ["//visibility:public"],
)

my_go_library(
    name = "a",
    embed = [":a_go_proto"],
    importpath = "example.com/mapkind/a",
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "b/BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("//:my.bzl", "my_go_library")

proto_library(
    name = "b_proto",
    srcs = ["b.proto"],
    visibility = ["//visibility:public"],
    deps = ["//a:a_proto"],
)

go_proto_library(
    name = "b_go_proto",
    importpath = "example.com/mapkind/b",
    proto = ":b_proto",
    visibility = ["//visibility:public"],
    deps = ["//a"],
)

my_go_library(
    name = "b",
    embed = [":b_go_proto"],
    importpath = "example.com/mapkind/b",
    visibility = ["//visibility:public"],
)
`,
		},
	})
}

// TestMinimalModuleCompatibilityAliases checks that importpath_aliases
// are emitted for go_libraries when needed. This can't easily be checked
// in language/go because the generator tests don't support running at
// the repository root or with additional flags, both of which are required.
func TestMinimalModuleCompatibilityAliases(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path:    "go.mod",
			Content: "module example.com/foo/v2",
		}, {
			Path:    "foo.go",
			Content: "package foo",
		}, {
			Path:    "bar/bar.go",
			Content: "package bar",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update", "-repo_root", dir, "-go_prefix", "example.com/foo/v2", "-go_repository_mode", "-go_repository_module_mode"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo/v2",
    importpath_aliases = ["example.com/foo"],
    visibility = ["//visibility:public"],
)
`,
		}, {
			Path: "bar/BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bar",
    srcs = ["bar.go"],
    importpath = "example.com/foo/v2/bar",
    importpath_aliases = ["example.com/foo/bar"],
    visibility = ["//visibility:public"],
)
`,
		},
	})
}

// TestGoImportVisibility checks that submodules implicitly declared with
// go_repository rules in the repo config file (WORKSPACE) have visibility
// for rules generated in internal directories where appropriate.
// Verifies #619.
func TestGoImportVisibility(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
go_repository(
		name = "com_example_m_logging",
    importpath = "example.com/m/logging",
)
`,
		}, {
			Path:    "internal/version/version.go",
			Content: "package version",
		}, {
			Path:    "internal/version/version_test.go",
			Content: "package version",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update", "-go_prefix", "example.com/m"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{{
		Path: "internal/version/BUILD.bazel",
		Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "version",
    srcs = ["version.go"],
    importpath = "example.com/m/internal/version",
    visibility = [
        "//:__subpackages__",
        "@com_example_m_logging//:__subpackages__",
    ],
)

go_test(
    name = "version_test",
    srcs = ["version_test.go"],
    embed = [":version"],
)
`,
	}})
}

// TestGoInternalVisibility_TopLevel checks that modules that are
// named internal/ expand visibility to repos that have a sibling
// importpath.
//
// Verifies #960
func TestGoInternalVisibility_TopLevel(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path:    "WORKSPACE",
			Content: `go_repository(name="org_modernc_ccgo", importpath="modernc.org/ccgo")`,
		}, {
			Path:    "BUILD.bazel",
			Content: `# gazelle:prefix modernc.org/internal`,
		}, {
			Path:    "internal.go",
			Content: "package internal",
		}, {
			Path:    "buffer/buffer.go",
			Content: "package buffer",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix modernc.org/internal

go_library(
    name = "internal",
    srcs = ["internal.go"],
    importpath = "modernc.org/internal",
    visibility = [
        "//:__subpackages__",
        "@org_modernc_ccgo//:__subpackages__",
    ],
)
`,
		},
		{
			Path: "buffer/BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "buffer",
    srcs = ["buffer.go"],
    importpath = "modernc.org/internal/buffer",
    visibility = [
        "//:__subpackages__",
        "@org_modernc_ccgo//:__subpackages__",
    ],
)
`,
		},
	})
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

func TestMatchProtoLibrary(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "proto/BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
# gazelle:prefix example.com/foo

proto_library(
	name = "existing_proto",
	srcs = ["foo.proto"],
)
`,
		},
		{
			Path:    "proto/foo.proto",
			Content: `syntax = "proto3";`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "proto/BUILD.bazel",
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
# gazelle:prefix example.com/foo

load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "existing_proto",
    srcs = ["foo.proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "existing_go_proto",
    importpath = "example.com/foo",
    proto = ":existing_proto",
    visibility = ["//visibility:public"],
)

go_library(
    name = "foo",
    embed = [":existing_go_proto"],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
)`,
		},
	})
}

func TestConfigLang(t *testing.T) {
	// Gazelle is run with "-lang=proto".
	files := []testtools.FileSpec{
		{Path: "WORKSPACE"},

		// Verify that Gazelle does not create a BUILD file.
		{Path: "foo/foo.go", Content: "package foo"},

		// Verify that Gazelle only creates the proto rule.
		{Path: "pb/pb.go", Content: "package pb"},
		{Path: "pb/pb.proto", Content: `syntax = "proto3";`},

		// Verify that Gazelle does create a BUILD file, because of the override.
		{Path: "bar/BUILD.bazel", Content: "# gazelle:lang"},
		{Path: "bar/bar.go", Content: "package bar"},
		{Path: "baz/BUILD.bazel", Content: "# gazelle:lang go,proto"},
		{Path: "baz/baz.go", Content: "package baz"},

		// Verify that Gazelle does not index go_library rules in // or //baz/protos.
		// In those directories, lang is set to proto by flag and directive, respectively.
		// Confirm it does index and resolve a rule in a directory where go is activated.
		{Path: "invisible1.go", Content: "package invisible1"},
		{Path: "BUILD.bazel", Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix root

go_library(
    name = "go_default_library",
    srcs = ["invisible1.go"],
    importpath = "root",
    visibility = ["//visibility:public"],
)
`},
		{Path: "baz/protos/invisible2.go", Content: "package invisible2"},
		{Path: "baz/protos/BUILD.bazel", Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:lang proto
# gazelle:prefix github.com/rule_indexing/invisible2

go_library(
    name = "go_default_library",
    srcs = ["invisible2.go"],
    importpath = "github.com/rule_indexing/invisible2",
    visibility = ["//visibility:public"],
)
`},
		{Path: "visible/visible.go", Content: "package visible"},
		{Path: "visible/BUILD.bazel", Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:lang go,proto
# gazelle:prefix github.com/rule_indexing/visible

go_library(
    name = "go_default_library",
    srcs = ["visible.go"],
    importpath = "github.com/rule_indexing/visible",
    visibility = ["//visibility:public"],
)
`},
		{Path: "baz/test_no_index/test_no_index.go", Content: `
package test_no_index

import (
	_ "github.com/rule_indexing/invisible1"
	_ "github.com/rule_indexing/invisible2"
	_ "github.com/rule_indexing/visible"
)
`},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"-lang", "proto"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path:     filepath.Join("foo", "BUILD.bazel"),
			NotExist: true,
		},
		{
			Path: filepath.Join("pb", "BUILD.bazel"),
			Content: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")

proto_library(
    name = "pb_proto",
    srcs = ["pb.proto"],
    visibility = ["//visibility:public"],
)`,
		},
		{
			Path: filepath.Join("bar", "BUILD.bazel"),
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:lang

go_library(
    name = "go_default_library",
    srcs = ["bar.go"],
    importpath = "root/bar",
    visibility = ["//visibility:public"],
)`,
		},
		{
			Path: filepath.Join("baz", "BUILD.bazel"),
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:lang go,proto

go_library(
    name = "go_default_library",
    srcs = ["baz.go"],
    importpath = "root/baz",
    visibility = ["//visibility:public"],
)`,
		},

		{Path: "baz/test_no_index/BUILD.bazel", Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["test_no_index.go"],
    importpath = "root/baz/test_no_index",
    visibility = ["//visibility:public"],
    deps = [
        "//visible:go_default_library",
        "@com_github_rule_indexing_invisible1//:go_default_library",
        "@com_github_rule_indexing_invisible2//:go_default_library",
    ],
)
`},
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

func TestGoGenerateProto(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "proto/BUILD.bazel",
			Content: `# gazelle:go_generate_proto false
# gazelle:prefix example.com/proto
`,
		},
		{
			Path:    "proto/foo.proto",
			Content: `syntax = "proto3";`,
		},
		{
			Path:    "proto/foo.pb.go",
			Content: "package proto",
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "proto/BUILD.bazel",
			Content: `load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:go_generate_proto false
# gazelle:prefix example.com/proto

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "proto",
    srcs = ["foo.pb.go"],
    importpath = "example.com/proto",
    visibility = ["//visibility:public"],
)`,
		},
	})
}

func TestGoMainLibraryRemoved(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "BUILD.bazel",
			Content: `
# gazelle:prefix example.com
# gazelle:go_naming_convention import
`,
		},
		{
			Path: "cmd/foo/BUILD.bazel",
			Content: `load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
		name = "foo_lib",
		srcs = ["foo.go"],
		importpath = "example.com/cmd/foo",
		visibility = ["//visibility:private"],
)

go_binary(
		name = "foo",
		embed = [":foo_lib"],
		visibility = ["//visibility:public"],
)
`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	args := []string{"update"}
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path:    "cmd/foo/BUILD.bazel",
			Content: "",
		},
	})
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

func TestExternalOnly(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "foo/foo.go",
			Content: `package foo
import _ "golang.org/x/baz"
`,
		},
		{
			Path: "foo/foo_test.go",
			Content: `package foo_test
import _ "golang.org/x/baz"
import _ "example.com/foo"
`,
		},
		{
			Path: "foo/BUILD.bazel",
			Content: `# gazelle:prefix example.com/foo
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
    deps = ["@org_golang_x_baz//:go_default_library"],
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)`,
		},
	}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	var args []string
	if err := runGazelle(dir, args); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "foo/BUILD.bazel",
			Content: `# gazelle:prefix example.com/foo
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
    deps = ["@org_golang_x_baz//:go_default_library"],
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    deps = [
        ":foo",
        "@org_golang_x_baz//:go_default_library",
    ],
)`,
		},
	})
}

func TestFindRulesGoVersionWithWORKSPACE(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "7b9bbe3ea1fccb46dcfa6c3f3e29ba7ec740d8733370e21cdc8937467b4a4349",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.22.4/rules_go-v0.22.4.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.22.4/rules_go-v0.22.4.tar.gz",
    ],
)
`,
		},
		{
			Path: "foo_illumos.go",
			Content: `
// illumos not supported in rules_go v0.22.4
package foo
`,
		},
		{
			Path: "BUILD.bazel",
			Content: `
# gazelle:prefix example.com/foo
`,
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"update"}); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
# gazelle:prefix example.com/foo
`,
		},
	})
}

func TestPlatformSpecificEmbedsrcs(t *testing.T) {
	files := []testtools.FileSpec{
		{
			Path: "WORKSPACE",
		},
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix example.com/foo

go_library(
    name = "foo",
    embedsrcs = ["deleted.txt"],
    importpath = "example.com/foo",
    srcs = ["foo.go"],
)
`,
		},
		{
			Path: "foo.go",
			Content: `
// +build windows

package foo

import _ "embed"

//go:embed windows.txt
var s string
`,
		},
		{
			Path: "windows.txt",
		},
	}

	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()

	if err := runGazelle(dir, []string{"update"}); err != nil {
		t.Fatal(err)
	}

	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "BUILD.bazel",
			Content: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix example.com/foo

go_library(
    name = "foo",
    srcs = ["foo.go"],
    embedsrcs = select({
        "@io_bazel_rules_go//go/platform:windows": [
            "windows.txt",
        ],
        "//conditions:default": [],
    }),
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
)
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
