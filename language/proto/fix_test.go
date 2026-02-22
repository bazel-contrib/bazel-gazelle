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

package proto

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

type fixTestCase struct {
	desc                 string
	protobufModuleName   string
	rulesProtoModuleName string
	old                  string
	want                 string
}

func TestFix(t *testing.T) {
	for _, tc := range []fixTestCase{
		{
			desc:               "switch from @rules_proto to @protobuf when protobuf module is present",
			protobufModuleName: "protobuf",
			old: `
load("@rules_proto//proto:defs.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
			want: `
load("@protobuf//bazel:proto_library.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
		},
		{
			desc: "does not remove @rules_proto load when no protobuf module",
			old: `
load("@rules_proto//proto:defs.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
			want: `
load("@rules_proto//proto:defs.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
		},
		{
			desc:               "multiple symbols loaded from @rules_proto",
			protobufModuleName: "protobuf",
			old: `
load("@rules_proto//proto:defs.bzl", "proto_library", "ProtoInfo")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
			want: `
load("@protobuf//bazel:proto_library.bzl", "proto_library")
load("@protobuf//bazel/common:proto_info.bzl", "ProtoInfo")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
		},
		{
			desc:               "preserve other load statements",
			protobufModuleName: "protobuf",
			old: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@rules_proto//proto:defs.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

go_library(
    name = "foo",
    srcs = ["foo.go"],
)
`,
			want: `
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@protobuf//bazel:proto_library.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

go_library(
    name = "foo",
    srcs = ["foo.go"],
)
`,
		},
		{
			desc:               "no-op when no @rules_proto load",
			protobufModuleName: "protobuf",
			old: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
			want: `
load("@com_google_protobuf//bazel:proto_library.bzl", "proto_library")

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
		},
		{
			desc:               "custom protobuf module name",
			protobufModuleName: "my_protobuf",
			old: `
load("@rules_proto//proto:defs.bzl", "proto_library")
`,
			want: `
load("@my_protobuf//bazel:proto_library.bzl", "proto_library")
`,
		},
		{
			desc:                 "custom rules_proto module name",
			protobufModuleName:   "protobuf",
			rulesProtoModuleName: "my_rules_proto",
			old: `
load("@my_rules_proto//proto:defs.bzl", "proto_library")
`,
			want: `
load("@protobuf//bazel:proto_library.bzl", "proto_library")
`,
		},
		{
			desc:               "drop an unknown symbol loaded from @rules_proto",
			protobufModuleName: "protobuf",
			old: `
load("@rules_proto//proto:defs.bzl", "proto_library", "unknown_symbol")
`,
			want: `
load("@protobuf//bazel:proto_library.bzl", "proto_library")
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			testFix(t, tc)
		})
	}
}

func testFix(t *testing.T, tc fixTestCase) {
	f, err := rule.LoadData(filepath.Join("old", "BUILD.bazel"), "", []byte(tc.old))
	if err != nil {
		t.Fatalf("%s: parse error: %v", tc.desc, err)
	}

	c := config.New()
	c.ModuleToApparentName = func(moduleName string) string {
		switch moduleName {
		case "protobuf":
			return tc.protobufModuleName
		case "rules_proto":
			return tc.rulesProtoModuleName
		default:
			return ""
		}
	}

	// Strip leading newline, added for readability
	want := strings.TrimPrefix(tc.want, "\n")

	NewLanguage().Fix(c, f)
	if got := string(f.Format()); got != want {
		t.Errorf("%s:\ngot:\n%s\nwant:\n%s", tc.desc, got, want)
	}
}
