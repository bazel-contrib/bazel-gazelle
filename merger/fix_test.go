/* Copyright 2022 The Bazel Authors. All rights reserved.

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

package merger_test

import (
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"github.com/google/go-cmp/cmp"
)

func TestFixLoads(t *testing.T) {
	knownLoads := []rule.LoadInfo{
		{
			Name: "@bar",
			Symbols: []string{
				"magic_macro",
				"magic_struct",
			},
		},
		{
			Name: "@foo",
			Symbols: []string{
				"foo_binary",
				"foo_library",
			},
		},
		{
			Name: "@foo",
			Symbols: []string{
				"foo_test",
			},
		},
		{
			Name: "@bazel_tools//tools/build_defs/repo:utils.bzl",
			Symbols: []string{
				"maybe",
			},
		},
		{
			Name: "@bazel_skylib//lib:selects.bzl",
			Symbols: []string{
				"selects",
			},
		},
	}

	type testCase struct {
		input string
		want  string
	}

	for name, tc := range map[string]testCase{
		"correct": {
			input: `load("@foo", "foo_binary", "foo_library")

maybe = "foo_name"

foo_binary(name = maybe)

foo_library(name = "a_lib")
`,
			want: `load("@foo", "foo_binary", "foo_library")

maybe = "foo_name"

foo_binary(name = maybe)

foo_library(name = "a_lib")
`,
		},
		"correct with macro": {
			input: `load("@bar", "magic_macro")
load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(
	name = "a_lib",
	deps = [
		"//a/b:c",
		magic_macro("baz"),
	],
)
		`,
			want: `load("@bar", "magic_macro")
load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(
    name = "a_lib",
    deps = [
        "//a/b:c",
        magic_macro("baz"),
    ],
)
`,
		},
		"missing macro load": {
			input: `load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(
    name = "a_lib",
    deps = [
        "//a/b:c",
        magic_macro("baz"),
    ],
)
`,
			want: `load("@bar", "magic_macro")
load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(
    name = "a_lib",
    deps = [
        "//a/b:c",
        magic_macro("baz"),
    ],
)
`,
		},
		"unused macro": {
			input: `load("@bar", "magic_macro")
			load("@foo", "foo_binary")

foo_binary(name = "a")
`,
			want: `load("@foo", "foo_binary")

foo_binary(name = "a")
`,
		},
		"missing kind load symbol": {
			input: `load("@foo", "foo_binary")

foo_binary(name = "a")

foo_library(name = "a_lib")
`,
			want: `load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(name = "a_lib")
`,
		},
		"missing kind load": {
			input: `foo_binary(name = "a")

foo_library(name = "a_lib")
`,
			want: `load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(name = "a_lib")
`,
		},
		"missing wrapper and wrapped kind load symbol": {
			input: `maybe(
    foo_binary,
    name = "a",
)
`,
			want: `load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")
load("@foo", "foo_binary")

maybe(
    foo_binary,
    name = "a",
)
`,
		},
		"unused kind load symbol": {
			input: `load("@foo", "foo_binary", "foo_library", "foo_test")

foo_binary(name = "a")

foo_library(name = "a_lib")
`,
			want: `load("@foo", "foo_binary", "foo_library")

foo_binary(name = "a")

foo_library(name = "a_lib")
`,
		},
		"struct macro": {
			input: `selects.config_setting_group(
    name = "a",
    match_any = [
        "//:config_a",
        "//:config_b",
    ],
)
`,
			want: `load("@bazel_skylib//lib:selects.bzl", "selects")

selects.config_setting_group(
    name = "a",
    match_any = [
        "//:config_a",
        "//:config_b",
    ],
)
`,
		},
		"struct member": {
			input: `foo_binary(
    name = "a",
    field = magic_struct.member,
)
`,
			want: `load("@bar", "magic_struct")
load("@foo", "foo_binary")

foo_binary(
    name = "a",
    field = magic_struct.member,
)
`,
		},
		"assignment expr rhs": {
			input: `foo_binary(
    name = "a",
    field = magic_macro,
)
`,
			want: `load("@bar", "magic_macro")
load("@foo", "foo_binary")

foo_binary(
    name = "a",
    field = magic_macro,
)
`,
		},
		"assignment expr rhs list": {
    		input: `foo_binary(
    name = "a",
    field = [magic_macro],
)
`,
    		want: `load("@bar", "magic_macro")
load("@foo", "foo_binary")

foo_binary(
    name = "a",
    field = [magic_macro],
)
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := rule.LoadData("", "", []byte(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			merger.FixLoads(f, knownLoads)
			f.Sync()

			want := strings.TrimSpace(tc.want)
			got := strings.TrimSpace(string(bzl.FormatWithoutRewriting(f.File)))
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("FixLoads() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFixLoadsSortsLoads(t *testing.T) {
	knownLoads := []rule.LoadInfo{
		{
			Name:    "@rules_python//python:defs.bzl",
			Symbols: []string{"py_binary", "py_library"},
		},
		{
			Name:    "@bazel_skylib//lib:selects.bzl",
			Symbols: []string{"selects"},
		},
		{
			Name:    "//tools:cc.bzl",
			Symbols: []string{"cc_library"},
		},
		{
			Name:    "//tools:py.bzl",
			Symbols: []string{"py_library"},
		},
	}

	type testCase struct {
		input string
		want  string
	}

	for name, tc := range map[string]testCase{
		"existing loads are sorted": {
			input: `load("//tools:py.bzl", "py_library")
load("@rules_python//python:defs.bzl", "py_binary")
load("//tools:cc.bzl", "cc_library")

py_binary(name = "a")

py_library(name = "b")

cc_library(name = "c")
`,
			want: `load("@rules_python//python:defs.bzl", "py_binary")
load("//tools:cc.bzl", "cc_library")
load("//tools:py.bzl", "py_library")

py_binary(name = "a")

py_library(name = "b")

cc_library(name = "c")
`,
		},
		"inserted loads are sorted with existing": {
			input: `load("//tools:py.bzl", "py_library")

py_library(name = "b")

selects.config_setting_group(
    name = "a",
    match_any = ["//:x"],
)
`,
			want: `load("@bazel_skylib//lib:selects.bzl", "selects")
load("//tools:py.bzl", "py_library")

py_library(name = "b")

selects.config_setting_group(
    name = "a",
    match_any = ["//:x"],
)
`,
		},
		"multiple inserted loads are sorted": {
			input: `py_binary(name = "a")

py_library(name = "b")

cc_library(name = "c")
`,
			want: `load("@rules_python//python:defs.bzl", "py_binary")
load("//tools:cc.bzl", "cc_library")
load("//tools:py.bzl", "py_library")

py_binary(name = "a")

py_library(name = "b")

cc_library(name = "c")
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := rule.LoadData("", "", []byte(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			merger.FixLoads(f, knownLoads)
			f.Sync()

			want := strings.TrimSpace(tc.want)
			got := strings.TrimSpace(string(bzl.FormatWithoutRewriting(f.File)))
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("FixLoads() sorting mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
