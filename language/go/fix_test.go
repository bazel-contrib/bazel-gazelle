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

package golang

import (
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

type fixTestCase struct {
	desc, old, want  string
	namingConvention namingConvention
}

func TestFixFile(t *testing.T) {
	for _, tc := range []fixTestCase{
		// migrateNamingConvention tests
		{
			desc:             "go_naming_convention=go_default_library -> import for lib",
			namingConvention: importNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
		},
		{
			desc:             "go_naming_convention=go_default_library -> import for bin",
			namingConvention: importNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_binary(
    name = "foo",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    importpath = "example.com/foo",
    srcs = ["foo.go"],
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_binary(
    name = "foo",
    embed = [":foo_lib"],
)

go_library(
    name = "foo_lib",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo_lib"],
)
`,
		},
		{
			desc:             "go_naming_convention=go_default_library -> import conflict",
			namingConvention: importNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load(":build_defs.bzl", "x_binary")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:private"],
)

x_binary(
    name = "foo",
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load(":build_defs.bzl", "x_binary")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:private"],
)

x_binary(
    name = "foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
		},
		{
			desc:             "go_naming_convention=import -> go_default_library for lib",
			namingConvention: goDefaultLibraryNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
		},
		{
			desc:             "go_naming_convention=import -> go_default_library for bin",
			namingConvention: goDefaultLibraryNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_binary(
    name = "foo",
    embed = [":foo_lib"],
)

go_library(
    name = "foo_lib",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo_lib"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_binary(
    name = "foo",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
		},
		{
			desc:             "go_naming_convention=go_default_library -> import_alias for lib",
			namingConvention: importAliasNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:private"],
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:private"],
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
		},
		{
			desc:             "go_naming_convention=import_alias -> go_default_library for lib",
			namingConvention: goDefaultLibraryNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
		},
		{
			desc:             "go_naming_convention=go_default_library -> import_alias for bin",
			namingConvention: importAliasNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_binary(
    name = "foo",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_binary(
    name = "foo",
    embed = [":foo_lib"],
)

go_library(
    name = "foo_lib",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo_lib"],
)
`,
		},
		{
			desc:             "go_naming_convention=import -> import_alias for lib",
			namingConvention: importAliasNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
		},
		{
			desc:             "go_naming_convention import_alias -> import for lib",
			namingConvention: importNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
		},
		{
			desc:             "go_naming_convention=import -> import for lib",
			namingConvention: importNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
		},
		{
			desc:             "go_naming_convention go_default_library -> go_default_library for lib",
			namingConvention: goDefaultLibraryNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:private"],
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:private"],
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
		},
		{
			desc:             "go_naming_convention=import_alias -> import_alias for lib",
			namingConvention: importAliasNamingConvention,
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "foo",
    srcs = ["foo.go"],
    importpath = "example.com/foo",
)

go_test(
    name = "foo_test",
    srcs = ["foo_test.go"],
    embed = [":foo"],
)
`,
		},
		{
			// migrateLibraryEmbed tests
			desc: "library migrated to embed",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    library = ":go_default_library",
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["foo.go"],
)

go_test(
    name = "go_default_test",
    srcs = ["foo_test.go"],
    embed = [":go_default_library"],
)
`,
		},
		{
			// verifies #211
			desc: "other library not migrated",
			old: `
gomock(
    name = "stripe_mock",
    out = "stripe_mock_test.go",
    interfaces = [
        "stSubscriptions",
        "stCustomers",
    ],
    library = ":go_default_library",
    package = "main",
    source = "stripe.go",
)
`,
			want: `
gomock(
    name = "stripe_mock",
    out = "stripe_mock_test.go",
    interfaces = [
        "stSubscriptions",
        "stCustomers",
    ],
    library = ":go_default_library",
    package = "main",
    source = "stripe.go",
)
`,
		},
		// migrateGrpcCompilers tests
		{
			desc: "go_grpc_library migrated to compilers",
			old: `load("@io_bazel_rules_go//proto:def.bzl", "go_grpc_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    visibility = ["//visibility:public"],
)

go_grpc_library(
    name = "foo_go_proto",
    importpath = "example.com/repo",
    proto = ":foo_proto",
    visibility = ["//visibility:public"],
)
`,
			want: `load("@io_bazel_rules_go//proto:def.bzl", "go_grpc_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "foo_go_proto",
    compilers = ["@io_bazel_rules_go//proto:go_grpc"],
    importpath = "example.com/repo",
    proto = ":foo_proto",
    visibility = ["//visibility:public"],
)
`,
		},
		// flattenSrcs tests
		{
			desc: "flatten srcs",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "gen.go",
    ] + select({
        "@io_bazel_rules_go//platform:darwin_amd64": [
            # darwin
            "foo.go", # keep
        ],
        "@io_bazel_rules_go//platform:linux_amd64": [
            # linux
            "foo.go", # keep
        ],
    }),
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        # darwin
        # linux
        "foo.go",  # keep
        "gen.go",
    ],
)
`,
		},
		// squashCgoLibrary tests
		{
			desc: "no cgo_library",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
)
`,
		},
		{
			desc: "non-default cgo_library not removed",
			old: `load("@io_bazel_rules_go//go:def.bzl", "cgo_library")

cgo_library(
    name = "something_else",
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "cgo_library")

cgo_library(
    name = "something_else",
)
`,
		},
		{
			desc: "unlinked cgo_library removed",
			old: `load("@io_bazel_rules_go//go:def.bzl", "cgo_library", "go_library")

go_library(
    name = "go_default_library",
    library = ":something_else",
)

cgo_library(
    name = "cgo_default_library",
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "cgo_library", "go_library")

go_library(
    name = "go_default_library",
    cgo = True,
)
`,
		},
		{
			desc: "cgo_library replaced with go_library",
			old: `load("@io_bazel_rules_go//go:def.bzl", "cgo_library")

# before comment
cgo_library(
    name = "cgo_default_library",
    cdeps = ["cdeps"],
    clinkopts = ["clinkopts"],
    copts = ["copts"],
    data = ["data"],
    deps = ["deps"],
    gc_goopts = ["gc_goopts"],
    srcs = [
        "foo.go"  # keep
    ],
    visibility = ["//visibility:private"],
)
# after comment
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "cgo_library")

# before comment
go_library(
    name = "go_default_library",
    srcs = [
        "foo.go",  # keep
    ],
    cdeps = ["cdeps"],
    cgo = True,
    clinkopts = ["clinkopts"],
    copts = ["copts"],
    data = ["data"],
    gc_goopts = ["gc_goopts"],
    visibility = ["//visibility:private"],
    deps = ["deps"],
)
# after comment
`,
		},
		{
			desc: "cgo_library merged with go_library",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

# before go_library
go_library(
    name = "go_default_library",
    srcs = ["pure.go"],
    deps = ["pure_deps"],
    data = ["pure_data"],
    gc_goopts = ["pure_gc_goopts"],
    library = ":cgo_default_library",
    cgo = False,
)
# after go_library

# before cgo_library
cgo_library(
    name = "cgo_default_library",
    srcs = ["cgo.go"],
    deps = ["cgo_deps"],
    data = ["cgo_data"],
    gc_goopts = ["cgo_gc_goopts"],
    copts = ["copts"],
    cdeps = ["cdeps"],
)
# after cgo_library
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

# before go_library
# before cgo_library
go_library(
    name = "go_default_library",
    srcs = [
        "cgo.go",
        "pure.go",
    ],
    cdeps = ["cdeps"],
    cgo = True,
    copts = ["copts"],
    data = [
        "cgo_data",
        "pure_data",
    ],
    gc_goopts = [
        "cgo_gc_goopts",
        "pure_gc_goopts",
    ],
    deps = [
        "cgo_deps",
        "pure_deps",
    ],
)
# after go_library
# after cgo_library
`,
		},
		// squashXtest tests
		{
			desc: "rename xtest",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_test")
go_test(
    name = "go_default_xtest",
    srcs = ["x_test.go"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
    name = "go_default_test",
    srcs = ["x_test.go"],
)
`,
		},
		{
			desc: "squash xtest",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
    name = "go_default_test",
    srcs = ["i_test.go"],
    deps = [
        ":i_dep",
        ":shared_dep",
    ],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_xtest",
    srcs = ["x_test.go"],
    deps = [
        ":x_dep",
        ":shared_dep",
    ],
    visibility = ["//visibility:public"],
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_test(
    name = "go_default_test",
    srcs = [
        "i_test.go",
        "x_test.go",
    ],
    visibility = ["//visibility:public"],
    deps = [
        ":i_dep",
        ":shared_dep",
        ":x_dep",
    ],
)
`,
		},
		// removeLegacyProto tests
		{
			desc: "current proto preserved",
			old: `load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

go_proto_library(
    name = "foo_go_proto",
    proto = ":foo_proto",
)
`,
			want: `load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

go_proto_library(
    name = "foo_go_proto",
    proto = ":foo_proto",
)
`,
		},
		{
			desc: "load and proto removed",
			old: `load("@io_bazel_rules_go//proto:go_proto_library.bzl", "go_proto_library")

go_proto_library(
    name = "go_default_library_protos",
    srcs = ["foo.proto"],
    visibility = ["//visibility:private"],
)
`,
			want: "",
		},
		{
			desc: "proto filegroup removed",
			old: `filegroup(
    name = "go_default_library_protos",
    srcs = ["foo.proto"],
)

go_proto_library(name = "foo_proto")
`,
			want: `go_proto_library(name = "foo_proto")
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			testFix(t, tc, func(f *rule.File) {
				c, langs, _ := testConfig(t,
					"-go_naming_convention="+tc.namingConvention.String(),
					"-go_prefix=example.com/foo",
				)
				c.ShouldFix = true
				for _, lang := range langs {
					lang.Fix(c, f)
				}
			})
		})
	}
}

func TestFixLoads(t *testing.T) {
	for _, tc := range []fixTestCase{
		{
			desc: "empty file",
			old:  "",
			want: "",
		}, {
			desc: "non-Go file",
			old: `load("@io_bazel_rules_intercal//intercal:def.bzl", "intercal_library")

intercal_library(
    name = "intercal_default_library",
    srcs = ["foo.ic"],
)
`,
			want: `load("@io_bazel_rules_intercal//intercal:def.bzl", "intercal_library")

intercal_library(
    name = "intercal_default_library",
    srcs = ["foo.ic"],
)
`,
		}, {
			desc: "add and remove loaded symbols",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(name = "go_default_library")

go_binary(name = "cmd")
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(name = "go_default_library")

go_binary(name = "cmd")
`,
		}, {
			desc: "consolidate load statements",
			old: `load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@io_bazel_rules_go//go:def.bzl", "go_test")

go_library(name = "go_default_library")

go_test(name = "go_default_test")
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(name = "go_default_library")

go_test(name = "go_default_test")
`,
		}, {
			desc: "new load statement",
			old: `go_library(
    name = "go_default_library",
)

go_embed_data(
    name = "data",
)
`,
			want: `load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
)

go_embed_data(
    name = "data",
)
`,
		}, {
			desc: "proto symbols",
			old: `go_proto_library(
    name = "foo_proto",
)

go_grpc_library(
    name = "bar_proto",
)
`,
			want: `load("@io_bazel_rules_go//proto:def.bzl", "go_grpc_library", "go_proto_library")

go_proto_library(
    name = "foo_proto",
)

go_grpc_library(
    name = "bar_proto",
)
`,
		}, {
			desc: "fixLoad doesn't touch other symbols or loads",
			old: `load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_embed_data",  # embed
    "go_test",
    foo = "go_binary",  # binary
)
load("@io_bazel_rules_go//proto:go_proto_library.bzl", "go_proto_library")

go_library(
    name = "go_default_library",
)
`,
			want: `load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_embed_data",  # embed
    "go_library",
    foo = "go_binary",  # binary
)
load("@io_bazel_rules_go//proto:go_proto_library.bzl", "go_proto_library")

go_library(
    name = "go_default_library",
)
`,
		}, {
			desc: "fixLoad doesn't touch loads from other files",
			old: `load(
    "@com_github_pubref_rules_protobuf//go:rules.bzl",
    "go_proto_library",
    go_grpc_library = "go_proto_library",
)

go_proto_library(
    name = "foo_go_proto",
)

grpc_proto_library(
    name = "bar_go_proto",
)
`,
			want: `load(
    "@com_github_pubref_rules_protobuf//go:rules.bzl",
    "go_proto_library",
    go_grpc_library = "go_proto_library",
)

go_proto_library(
    name = "foo_go_proto",
)

grpc_proto_library(
    name = "bar_go_proto",
)
`,
		}, {
			desc: "moved symbol",
			old: `
load("@io_bazel_rules_go//go:def.bzl", "go_repository")

go_repository(name = "foo")
`,
			want: `
load("@bazel_gazelle//:deps.bzl", "go_repository")

go_repository(name = "foo")
`,
		}, {
			desc: "moved symbols with others",
			old: `
load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_repository")

go_rules_dependencies()

go_repository(name = "foo")
`,
			want: `load("@bazel_gazelle//:deps.bzl", "go_repository")
load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies")

go_rules_dependencies()

go_repository(name = "foo")
`,
		}, {
			desc: "load go_repository",
			old: `
load("@io_bazel_rules_go//go:def.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

go_repository(name = "foo")
`,
			want: `load("@bazel_gazelle//:deps.bzl", "go_repository")
load("@io_bazel_rules_go//go:def.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

go_repository(name = "foo")
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			testFix(t, tc, func(f *rule.File) {
				merger.FixLoads(f, goLoadsForTesting)
			})
		})
	}
}

func testFix(t *testing.T, tc fixTestCase, fix func(*rule.File)) {
	f, err := rule.LoadData(filepath.Join("old", "BUILD.bazel"), "", []byte(tc.old))
	if err != nil {
		t.Fatalf("%s: parse error: %v", tc.desc, err)
	}
	fix(f)
	want := tc.want
	if len(want) > 0 && want[0] == '\n' {
		// Strip leading newline, added for readability
		want = want[1:]
	}
	if got := string(f.Format()); got != want {
		t.Fatalf("%s: got %s; want %s", tc.desc, got, want)
	}
}
