/* Copyright 2018 The Bazel Authors. All rights reserved.

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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"golang.org/x/tools/go/vcs"
)

func TestResolveGo(t *testing.T) {
	type buildFile struct {
		rel, content string
	}
	type testCase struct {
		desc             string
		index            []buildFile
		old              buildFile
		want             string
		skipIndex        bool
		namingConvention namingConvention
	}
	for _, tc := range []testCase{
		{
			desc: "std",
			index: []buildFile{{
				rel: "bad",
				content: `
go_library(
    name = "go_default_library",
    importpath = "fmt",
)
`,
			}},
			old: buildFile{
				content: `
go_binary(
    name = "dep",
    _imports = ["fmt"],
)
`,
			},
			want: `go_binary(name = "dep")`,
		}, {
			desc: "self_import",
			old: buildFile{content: `
go_library(
    name = "go_default_library",
    importpath = "foo",
    _imports = ["foo"],
)
`},
			want: `
go_library(
    name = "go_default_library",
    importpath = "foo",
)
`,
		}, {
			desc: "self_import_embed",
			old: buildFile{content: `
go_library(
    name = "a",
    embeds = [":b"],
    importpath = "x",
)

go_library(
    name = "b",
    importpath = "x",
    _imports = ["x"],
)
`},
			want: `
go_library(
    name = "a",
    embeds = [":b"],
    importpath = "x",
)

go_library(
    name = "b",
    importpath = "x",
)
`,
		}, {
			desc: "override",
			index: []buildFile{{
				content: `
# gazelle:resolve go go github.com/golang/protobuf/ptypes //:good
go_library(
    name = "bad",
    importpath = "github.com/golang/protobuf/ptypes",
)
`,
			}},
			old: buildFile{
				rel: "test",
				content: `
go_library(
    name = "a",
    importpath = "a",
    _imports = ["github.com/golang/protobuf/ptypes"],
)
`,
			},
			want: `
go_library(
    name = "a",
    importpath = "a",
    deps = ["//:good"],
)
`,
		}, {
			desc: "same_package",
			old: buildFile{content: `
go_library(
    name = "a",
    importpath = "foo",
)

go_binary(
    name = "b",
    _imports = ["foo"],
)
`},
			want: `
go_library(
    name = "a",
    importpath = "foo",
)

go_binary(
    name = "b",
    deps = [":a"],
)
`,
		}, {
			desc: "different_package",
			index: []buildFile{{
				rel: "a",
				content: `
go_library(
    name = "a_lib",
    importpath = "aa",
)
`,
			}},
			old: buildFile{
				rel: "b",
				content: `
go_binary(
    name = "bin",
    _imports = ["aa"],
)
`,
			},
			want: `
go_binary(
    name = "bin",
    deps = ["//a:a_lib"],
)
`,
		}, {
			desc: "multiple_rules_ambiguous",
			index: []buildFile{{
				rel: "foo",
				content: `
go_library(
    name = "a",
    importpath = "example.com/foo",
)

go_library(
    name = "b",
    importpath = "example.com/foo",
)
`,
			}},
			old: buildFile{
				content: `
go_binary(
    name = "bin",
    _imports = ["example.com/foo"],
)
`,
			},
			// an error should be reported, and no dependency should be emitted
			want: `go_binary(name = "bin")`,
		}, {
			desc: "vendor_not_visible",
			index: []buildFile{
				{
					rel: "",
					content: `
go_library(
    name = "root",
    importpath = "example.com/foo",
)
`,
				}, {
					rel: "a/vendor/foo",
					content: `
go_library(
    name = "vendored",
    importpath = "example.com/foo",
)
`,
				},
			},
			old: buildFile{
				rel: "b",
				content: `
go_binary(
    name = "bin",
    _imports = ["example.com/foo"],
)
`,
			},
			want: `
go_binary(
    name = "bin",
    deps = ["//:root"],
)
`,
		}, {
			desc: "vendor_supercedes_nonvendor",
			index: []buildFile{
				{
					rel: "",
					content: `
go_library(
    name = "root",
    importpath = "example.com/foo",
)
`,
				}, {
					rel: "vendor/foo",
					content: `
go_library(
    name = "vendored",
    importpath = "example.com/foo",
)
`,
				},
			},
			old: buildFile{
				rel: "sub",
				content: `
go_binary(
    name = "bin",
    _imports = ["example.com/foo"],
)
`,
			},
			want: `
go_binary(
    name = "bin",
    deps = ["//vendor/foo:vendored"],
)
`,
		}, {
			desc: "deep_vendor_shallow_vendor",
			index: []buildFile{
				{
					rel: "shallow/vendor",
					content: `
go_library(
    name = "shallow",
    importpath = "example.com/foo",
)
`,
				}, {
					rel: "shallow/deep/vendor",
					content: `
go_library(
    name = "deep",
    importpath = "example.com/foo",
)
`,
				},
			},
			old: buildFile{
				rel: "shallow/deep",
				content: `
go_binary(
    name = "bin",
    _imports = ["example.com/foo"],
)
`,
			},
			want: `
go_binary(
    name = "bin",
    deps = ["//shallow/deep/vendor:deep"],
)
`,
		}, {
			desc: "nested_vendor",
			index: []buildFile{
				{
					rel: "vendor/a",
					content: `
go_library(
    name = "a",
    importpath = "a",
)
`,
				}, {
					rel: "vendor/b/vendor/a",
					content: `
go_library(
    name = "a",
    importpath = "a",
)
`,
				},
			},
			old: buildFile{
				rel: "vendor/b/c",
				content: `
go_binary(
    name = "bin",
    _imports = ["a"],
)
`,
			},
			want: `
go_binary(
    name = "bin",
    deps = ["//vendor/b/vendor/a"],
)
`,
		}, {
			desc: "skip_self_embed",
			old: buildFile{
				content: `
go_library(
    name = "go_default_library",
    srcs = ["lib.go"],
    importpath = "example.com/repo/lib",
)

go_test(
    name = "go_default_test",
    embed = [":go_default_library"],
    _imports = ["example.com/repo/lib"],
)
`,
			},
			want: `
go_library(
    name = "go_default_library",
    srcs = ["lib.go"],
    importpath = "example.com/repo/lib",
)

go_test(
    name = "go_default_test",
    embed = [":go_default_library"],
)
`,
		}, {
			desc: "binary_embed",
			old: buildFile{content: `
go_library(
    name = "a",
    importpath = "a",
)

go_library(
    name = "b",
    embed = [":a"],
)

go_binary(
    name = "c",
    embed = [":a"],
    importpath = "a",
)

go_library(
    name = "d",
    _imports = ["a"],
)
`},
			want: `
go_library(
    name = "a",
    importpath = "a",
)

go_library(
    name = "b",
    embed = [":a"],
)

go_binary(
    name = "c",
    embed = [":a"],
    importpath = "a",
)

go_library(
    name = "d",
    deps = [":b"],
)
`,
		}, {
			desc: "gazelle_special",
			old: buildFile{content: `
go_library(
    name = "go_default_library",
    _imports = [
        "github.com/bazelbuild/bazel-gazelle/language",
        "github.com/bazelbuild/rules_go/go/tools/bazel",
    ],
)
`},
			want: `
go_library(
    name = "go_default_library",
    deps = [
        "@bazel_gazelle//language:go_default_library",
        "@io_bazel_rules_go//go/tools/bazel:go_default_library",
    ],
)
`,
		}, {
			desc:      "local_unknown",
			skipIndex: true,
			old: buildFile{content: `
go_binary(
    name = "bin",
    _imports = [
        "example.com/repo/resolve",
        "example.com/repo/resolve/sub",
    ],
)
`},
			want: `
go_binary(
    name = "bin",
    deps = [
        ":resolve",
        "//sub",
    ],
)
`,
		}, {
			desc: "local_relative",
			index: []buildFile{
				{
					rel: "a",
					content: `
go_library(
    name = "go_default_library",
    importpath = "example.com/repo/resolve/a",
)
`,
				},
				{
					rel: "a/b",
					content: `
go_library(
    name = "go_default_library",
    importpath = "example.com/repo/resolve/a/b",
)
`,
				},
				{
					rel: "c",
					content: `
go_library(
    name = "go_default_library",
    importpath = "example.com/repo/resolve/c",
)
`,
				},
			},
			old: buildFile{
				rel: "a",
				content: `
go_binary(
    name = "bin",
    _imports = [
        ".",
        "./b",
        "../c",
    ],
)
`,
			},
			want: `
go_binary(
    name = "bin",
    deps = [
        ":go_default_library",
        "//a/b:go_default_library",
        "//c:go_default_library",
    ],
)
`,
		}, {
			desc: "vendor_no_index",
			old: buildFile{content: `
go_binary(
    name = "bin",
    _imports = ["example.com/outside/prefix"],
)
`},
			want: `
go_binary(
    name = "bin",
    deps = ["//vendor/example.com/outside/prefix"],
)
`,
		}, {
			desc:             "vendor with go_naming_convention=import",
			namingConvention: importNamingConvention,
			old: buildFile{content: `
go_binary(
    name = "bin",
    _imports = ["example.com/outside/prefix"],
)
`},
			want: `
go_binary(
    name = "bin",
    deps = ["//vendor/example.com/outside/prefix"],
)
`,
		}, {
			desc: "test_and_library_not_indexed",
			index: []buildFile{{
				rel: "foo",
				content: `
go_test(
    name = "go_default_test",
    importpath = "example.com/foo",
)

go_binary(
    name = "cmd",
    importpath = "example.com/foo",
)
`,
			}},
			old: buildFile{content: `
go_binary(
    name = "bin",
    _imports = ["example.com/foo"],
)
`},
			want: `
go_binary(
    name = "bin",
    deps = ["//vendor/example.com/foo"],
)`,
		}, {
			desc: "proto_override",
			index: []buildFile{{
				rel: "",
				content: `
# gazelle:resolve proto go google/rpc/status.proto :good

proto_library(
    name = "bad_proto",
    srcs = ["google/rpc/status.proto"],
)

go_proto_library(
    name = "bad_go_proto",
    proto = ":bad_proto",
    importpath = "bad",
)
`,
			}},
			old: buildFile{
				rel: "test",
				content: `
go_proto_library(
    name = "dep_go_proto",
    importpath = "test",
    _imports = ["google/rpc/status.proto"],
)
`,
			},
			want: `
go_proto_library(
    name = "dep_go_proto",
    importpath = "test",
    deps = ["//:good"],
)
`,
		}, {
			desc: "proto_override_regexp",
			index: []buildFile{{
				rel: "",
				content: `
# gazelle:resolve_regexp proto go google/rpc/.*\.proto :good

proto_library(
    name = "bad_proto",
    srcs = ["google/rpc/status.proto"],
)

go_proto_library(
    name = "bad_go_proto",
    proto = ":bad_proto",
    importpath = "bad",
)
`,
			}},
			old: buildFile{
				rel: "test",
				content: `
go_proto_library(
    name = "dep_go_proto",
    importpath = "test",
    _imports = [
		"google/rpc/status.proto",
		"google/rpc/status1.proto",
		"google/rpc/status2.proto",
	],
)
`,
			},
			want: `
go_proto_library(
    name = "dep_go_proto",
    importpath = "test",
    deps = ["//:good"],
)
`,
		}, {
			desc: "proto_index",
			index: []buildFile{{
				rel: "sub",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["bar.proto"],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "example.com/foo",
    proto = ":foo_proto",
)

go_library(
    name = "embed",
    embed = [":foo_go_proto"],
    importpath = "example.com/foo",
)
`,
			}},
			old: buildFile{content: `
go_proto_library(
    name = "dep_proto",
    _imports = ["sub/bar.proto"],
)
`},
			want: `
go_proto_library(
    name = "dep_proto",
    deps = ["//sub:embed"],
)
`,
		}, {
			desc: "proto_embed",
			old: buildFile{content: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "example.com/repo/foo",
    proto = ":foo_proto",
)

go_library(
    name = "foo_embedder",
    embed = [":foo_go_proto"],
)

proto_library(
    name = "bar_proto",
    srcs = ["bar.proto"],
    _imports = ["foo.proto"],
)

go_proto_library(
    name = "bar_go_proto",
    proto = ":bar_proto",
    _imports = ["foo.proto"],
)
`},
			want: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "example.com/repo/foo",
    proto = ":foo_proto",
)

go_library(
    name = "foo_embedder",
    embed = [":foo_go_proto"],
)

proto_library(
    name = "bar_proto",
    srcs = ["bar.proto"],
    deps = [":foo_proto"],
)

go_proto_library(
    name = "bar_go_proto",
    proto = ":bar_proto",
    deps = [":foo_embedder"],
)
`,
		}, {
			desc: "proto_dedup",
			index: []buildFile{{
				rel: "sub",
				content: `
proto_library(
    name = "foo_proto",
    srcs = [
        "a.proto",
        "b.proto",
    ],
)

go_proto_library(
    name = "foo_go_proto",
    proto = ":foo_proto",
    importpath = "sub",
)
`,
			}},
			old: buildFile{content: `
go_proto_library(
    name = "dep_proto",
    _imports = [
        "sub/a.proto",
        "sub/b.proto",
    ],
)
`},
			want: `
go_proto_library(
    name = "dep_proto",
    deps = ["//sub:foo_go_proto"],
)
`,
		}, {
			desc: "proto_wkt_cross_resolve",
			old: buildFile{content: `
go_proto_library(
    name = "wkts_go_proto",
    _imports = [
        "google/protobuf/any.proto",
        "google/protobuf/api.proto",
        "google/protobuf/compiler/plugin.proto",
        "google/protobuf/descriptor.proto",
        "google/protobuf/duration.proto",
        "google/protobuf/empty.proto",
        "google/protobuf/field_mask.proto",
        "google/protobuf/source_context.proto",
        "google/protobuf/struct.proto",
        "google/protobuf/timestamp.proto",
        "google/protobuf/type.proto",
        "google/protobuf/wrappers.proto",
   ],
)

go_library(
    name = "wkts_go_lib",
    _imports = [
        "github.com/golang/protobuf/ptypes/any",
        "google.golang.org/genproto/protobuf/api",
        "github.com/golang/protobuf/protoc-gen-go/descriptor",
        "github.com/golang/protobuf/ptypes/duration",
        "github.com/golang/protobuf/ptypes/empty",
        "google.golang.org/genproto/protobuf/field_mask",
        "google.golang.org/genproto/protobuf/source_context",
        "github.com/golang/protobuf/ptypes/struct",
        "github.com/golang/protobuf/ptypes/timestamp",
        "github.com/golang/protobuf/ptypes/wrappers",
        "github.com/golang/protobuf/protoc-gen-go/plugin",
        "google.golang.org/genproto/protobuf/ptype",
   ],
)
`},
			want: `
go_proto_library(name = "wkts_go_proto")

go_library(
    name = "wkts_go_lib",
    deps = [
        "//vendor/github.com/golang/protobuf/protoc-gen-go/descriptor",
        "//vendor/github.com/golang/protobuf/protoc-gen-go/plugin",
        "//vendor/github.com/golang/protobuf/ptypes/any",
        "//vendor/github.com/golang/protobuf/ptypes/duration",
        "//vendor/github.com/golang/protobuf/ptypes/empty",
        "//vendor/github.com/golang/protobuf/ptypes/struct",
        "//vendor/github.com/golang/protobuf/ptypes/timestamp",
        "//vendor/github.com/golang/protobuf/ptypes/wrappers",
        "//vendor/google.golang.org/genproto/protobuf/api",
        "//vendor/google.golang.org/genproto/protobuf/field_mask",
        "//vendor/google.golang.org/genproto/protobuf/ptype",
        "//vendor/google.golang.org/genproto/protobuf/source_context",
    ],
)
`,
		}, {
			desc: "proto_self_import",
			old: buildFile{content: `
proto_library(
    name = "foo_proto",
    srcs = [
        "a.proto",
        "b.proto",
    ],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "foo",
    proto = ":foo_proto",
    _imports = ["a.proto"],
)

go_library(
    name = "go_default_library",
    embed = [":foo_go_proto"],
    importpath = "foo",
)
`},
			want: `
proto_library(
    name = "foo_proto",
    srcs = [
        "a.proto",
        "b.proto",
    ],
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "foo",
    proto = ":foo_proto",
)

go_library(
    name = "go_default_library",
    embed = [":foo_go_proto"],
    importpath = "foo",
)
`,
		}, {
			desc: "proto_import_prefix_and_strip_import_prefix",
			index: []buildFile{{
				rel: "sub",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["bar.proto"],
    import_prefix = "foo/",
    strip_import_prefix = "/sub",
)

go_proto_library(
    name = "foo_go_proto",
    importpath = "example.com/foo",
    proto = ":foo_proto",
)

go_library(
    name = "embed",
    embed = [":foo_go_proto"],
    importpath = "example.com/foo",
)
`,
			}},
			old: buildFile{content: `
go_proto_library(
    name = "dep_proto",
    _imports = ["foo/bar.proto"],
)
`},
			want: `
go_proto_library(
    name = "dep_proto",
    deps = ["//sub:embed"],
)
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c, langs, cexts := testConfig(
				t,
				"-go_prefix=example.com/repo/resolve",
				fmt.Sprintf("-go_naming_convention=%s", tc.namingConvention),
				"-external=vendored", fmt.Sprintf("-index=%v", !tc.skipIndex))
			mrslv := make(mapResolver)
			exts := make([]interface{}, 0, len(langs))
			for _, lang := range langs {
				for kind := range lang.Kinds() {
					mrslv[kind] = lang
				}
				exts = append(exts, lang)
			}
			ix := resolve.NewRuleIndex(mrslv.Resolver, exts...)
			rc := testRemoteCache(nil)

			for _, bf := range tc.index {
				buildPath := filepath.Join(filepath.FromSlash(bf.rel), "BUILD.bazel")
				f, err := rule.LoadData(buildPath, bf.rel, []byte(bf.content))
				if err != nil {
					t.Fatal(err)
				}
				if bf.rel == "" {
					for _, cext := range cexts {
						cext.Configure(c, "", f)
					}
				}
				for _, r := range f.Rules {
					ix.AddRule(c, r, f)
				}
			}
			buildPath := filepath.Join(filepath.FromSlash(tc.old.rel), "BUILD.bazel")
			f, err := rule.LoadData(buildPath, tc.old.rel, []byte(tc.old.content))
			if err != nil {
				t.Fatal(err)
			}
			imports := make([]interface{}, len(f.Rules))
			for i, r := range f.Rules {
				imports[i] = convertImportsAttr(r)
				ix.AddRule(c, r, f)
			}
			ix.Finish()
			for i, r := range f.Rules {
				mrslv.Resolver(r, "").Resolve(c, ix, rc, r, imports[i], label.New("", tc.old.rel, r.Name()))
			}
			f.Sync()
			got := strings.TrimSpace(string(bzl.Format(f.File)))
			want := strings.TrimSpace(tc.want)
			if got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestResolveDisableGlobal(t *testing.T) {
	c, langs, _ := testConfig(
		t,
		"-go_prefix=example.com/repo",
		"-proto=disable_global")
	exts := make([]interface{}, 0, len(langs))
	for _, lang := range langs {
		exts = append(exts, lang)
	}
	ix := resolve.NewRuleIndex(nil, exts...)
	ix.Finish()
	rc := testRemoteCache([]repo.Repo{
		{
			Name:     "com_github_golang_protobuf",
			GoPrefix: "github.com/golang/protobuf",
		}, {
			Name:     "org_golang_google_genproto",
			GoPrefix: "golang.org/google/genproto",
		},
	})
	gl := langs[1].(*goLang)
	oldContent := []byte(`
go_library(
    name = "go_default_library",
    importpath = "foo",
    _imports = [
        "github.com/golang/protobuf/ptypes/any",
        "google.golang.org/genproto/protobuf/api",
        "github.com/golang/protobuf/protoc-gen-go/descriptor",
        "github.com/golang/protobuf/ptypes/duration",
        "github.com/golang/protobuf/ptypes/empty",
        "google.golang.org/genproto/protobuf/field_mask",
        "google.golang.org/genproto/protobuf/source_context",
        "github.com/golang/protobuf/ptypes/struct",
        "github.com/golang/protobuf/ptypes/timestamp",
        "github.com/golang/protobuf/ptypes/wrappers",
        "github.com/golang/protobuf/protoc-gen-go/plugin",
        "google.golang.org/genproto/protobuf/ptype",
        "google.golang.org/genproto/googleapis/api/annotations",
        "google.golang.org/genproto/googleapis/rpc/status",
        "google.golang.org/genproto/googleapis/type/latlng",
        "github.com/golang/protobuf/jsonpb",
        "github.com/golang/protobuf/descriptor",
        "github.com/golang/protobuf/ptypes",
    ],
)
`)
	f, err := rule.LoadData("BUILD.bazel", "", oldContent)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range f.Rules {
		imports := convertImportsAttr(r)
		gl.Resolve(c, ix, rc, r, imports, label.New("", "", r.Name()))
	}
	f.Sync()
	got := strings.TrimSpace(string(bzl.Format(f.File)))
	want := strings.TrimSpace(`
go_library(
    name = "go_default_library",
    importpath = "foo",
    deps = [
        "@com_github_golang_protobuf//descriptor:go_default_library",
        "@com_github_golang_protobuf//jsonpb:go_default_library",
        "@com_github_golang_protobuf//protoc-gen-go/descriptor:go_default_library",
        "@com_github_golang_protobuf//protoc-gen-go/plugin:go_default_library",
        "@com_github_golang_protobuf//ptypes:go_default_library",
        "@com_github_golang_protobuf//ptypes/any:go_default_library",
        "@com_github_golang_protobuf//ptypes/duration:go_default_library",
        "@com_github_golang_protobuf//ptypes/empty:go_default_library",
        "@com_github_golang_protobuf//ptypes/struct:go_default_library",
        "@com_github_golang_protobuf//ptypes/timestamp:go_default_library",
        "@com_github_golang_protobuf//ptypes/wrappers:go_default_library",
        "@org_golang_google_genproto//googleapis/api/annotations:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/status:go_default_library",
        "@org_golang_google_genproto//googleapis/type/latlng:go_default_library",
        "@org_golang_google_genproto//protobuf/api:go_default_library",
        "@org_golang_google_genproto//protobuf/field_mask:go_default_library",
        "@org_golang_google_genproto//protobuf/ptype:go_default_library",
        "@org_golang_google_genproto//protobuf/source_context:go_default_library",
    ],
)
`)
	if got != want {
		t.Errorf("got:\n%s\nwant:%s", got, want)
	}
}

func TestResolveExternal(t *testing.T) {
	c, langs, _ := testConfig(
		t,
		"-go_prefix=example.com/local")
	gc := getGoConfig(c)
	ix := resolve.NewRuleIndex(nil)
	ix.Finish()
	gl := langs[1].(*goLang)
	for _, tc := range []struct {
		desc, importpath         string
		repos                    []repo.Repo
		moduleMode               bool
		depMode                  dependencyMode
		namingConvention         namingConvention
		namingConventionExternal namingConvention
		repoNamingConvention     map[string]namingConvention
		want                     string
	}{
		{
			desc:       "top",
			importpath: "example.com/repo",
			want:       "@com_example_repo//:go_default_library",
		}, {
			desc:             "top_import_naming_convention",
			namingConvention: goDefaultLibraryNamingConvention,
			repoNamingConvention: map[string]namingConvention{
				"com_example_repo": importNamingConvention,
			},
			importpath: "example.com/repo",
			want:       "@com_example_repo//:repo",
		}, {
			desc:             "top_import_alias_naming_convention",
			namingConvention: goDefaultLibraryNamingConvention,
			repoNamingConvention: map[string]namingConvention{
				"com_example_repo": importAliasNamingConvention,
			},
			importpath: "example.com/repo",
			want:       "@com_example_repo//:go_default_library",
		}, {
			desc:       "sub",
			importpath: "example.com/repo/lib",
			want:       "@com_example_repo//lib:go_default_library",
		}, {
			desc:             "sub_import_alias_naming_convention",
			namingConvention: importNamingConvention,
			repoNamingConvention: map[string]namingConvention{
				"com_example_repo": importAliasNamingConvention,
			},
			importpath: "example.com/repo/lib",
			want:       "@com_example_repo//lib",
		}, {
			desc: "custom_repo",
			repos: []repo.Repo{{
				Name:     "custom_repo_name",
				GoPrefix: "example.com/repo",
			}},
			importpath: "example.com/repo/lib",
			want:       "@custom_repo_name//lib:go_default_library",
		}, {
			desc: "custom_repo_import_naming_convention",
			repos: []repo.Repo{{
				Name:     "custom_repo_name",
				GoPrefix: "example.com/repo",
			}},
			repoNamingConvention: map[string]namingConvention{
				"custom_repo_name": importNamingConvention,
			},
			importpath: "example.com/repo/lib",
			want:       "@custom_repo_name//lib",
		}, {
			desc: "custom_repo_naming_convention_extern_import",
			repos: []repo.Repo{{
				Name:     "custom_repo_name",
				GoPrefix: "example.com/repo",
			}},
			namingConventionExternal: importNamingConvention,
			importpath:               "example.com/repo/lib",
			want:                     "@custom_repo_name//lib",
		}, {
			desc: "custom_repo_naming_convention_extern_default",
			repos: []repo.Repo{{
				Name:     "custom_repo_name",
				GoPrefix: "example.com/repo",
			}},
			namingConventionExternal: goDefaultLibraryNamingConvention,
			importpath:               "example.com/repo/lib",
			want:                     "@custom_repo_name//lib:go_default_library",
		}, {
			desc:       "qualified",
			importpath: "example.com/repo.git/lib",
			want:       "@com_example_repo_git//lib:go_default_library",
		}, {
			desc:       "domain",
			importpath: "example.com/lib",
			want:       "@com_example//lib:go_default_library",
		}, {
			desc:       "same_prefix",
			importpath: "example.com/local/ext",
			repos: []repo.Repo{{
				Name:     "local_ext",
				GoPrefix: "example.com/local/ext",
			}},
			want: "@local_ext//:go_default_library",
		}, {
			desc:       "module_mode_unknown",
			importpath: "example.com/repo/v2/foo",
			moduleMode: true,
			want:       "@com_example_repo_v2//foo:go_default_library",
		}, {
			desc:       "module_mode_known",
			importpath: "example.com/repo/v2/foo",
			repos: []repo.Repo{{
				Name:     "custom_repo",
				GoPrefix: "example.com/repo",
			}},
			moduleMode: true,
			want:       "@custom_repo//v2/foo:go_default_library",
		}, {
			desc:       "min_module_compat",
			importpath: "example.com/foo",
			repos: []repo.Repo{{
				Name:     "com_example_foo_v2",
				GoPrefix: "example.com/foo/v2",
			}},
			moduleMode: true,
			want:       "@com_example_foo_v2//:go_default_library",
		}, {
			desc:       "min_module_compat_both",
			importpath: "example.com/foo",
			repos: []repo.Repo{
				{
					Name:     "com_example_foo",
					GoPrefix: "example.com/foo",
				}, {
					Name:     "com_example_foo_v2",
					GoPrefix: "example.com/foo/v2",
				},
			},
			moduleMode: true,
			want:       "@com_example_foo//:go_default_library",
		}, {
			desc:       "static_mode_known",
			importpath: "example.com/repo/v2/foo",
			repos: []repo.Repo{{
				Name:     "custom_repo",
				GoPrefix: "example.com/repo",
			}},
			depMode: staticMode,
			want:    "@custom_repo//v2/foo:go_default_library",
		}, {
			desc:       "static_mode_unknown",
			importpath: "example.com/repo/v2/foo",
			depMode:    staticMode,
			want:       "",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gc.depMode = tc.depMode
			gc.moduleMode = tc.moduleMode
			gc.goNamingConvention = tc.namingConvention
			gc.goNamingConventionExternal = tc.namingConventionExternal
			gc.repoNamingConvention = tc.repoNamingConvention
			rc := testRemoteCache(tc.repos)
			r := rule.NewRule("go_library", "x")
			imports := rule.PlatformStrings{Generic: []string{tc.importpath}}
			gl.Resolve(c, ix, rc, r, imports, label.New("", "", "x"))
			deps := r.AttrStrings("deps")
			if tc.want == "" {
				if len(deps) != 0 {
					t.Fatalf("deps: got %d; want 0", len(deps))
				}
				return
			} else if len(deps) != 1 {
				t.Fatalf("deps: got %d; want 1", len(deps))
			}
			if deps[0] != tc.want {
				t.Errorf("got %s; want %s", deps[0], tc.want)
			}
		})
	}
}

func testRemoteCache(knownRepos []repo.Repo) *repo.RemoteCache {
	rc, _ := repo.NewRemoteCache(knownRepos)
	rc.RepoRootForImportPath = stubRepoRootForImportPath
	rc.HeadCmd = func(_, _ string) (string, error) {
		return "", fmt.Errorf("HeadCmd not supported in test")
	}
	rc.ModInfo = stubModInfo
	return rc
}

// stubRepoRootForImportPath is a stub implementation of vcs.RepoRootForImportPath
func stubRepoRootForImportPath(importPath string, verbose bool) (*vcs.RepoRoot, error) {
	if pathtools.HasPrefix(importPath, "example.com/repo.git") {
		return &vcs.RepoRoot{
			VCS:  vcs.ByCmd("git"),
			Repo: "https://example.com/repo.git",
			Root: "example.com/repo.git",
		}, nil
	}

	if pathtools.HasPrefix(importPath, "example.com/repo") {
		return &vcs.RepoRoot{
			VCS:  vcs.ByCmd("git"),
			Repo: "https://example.com/repo.git",
			Root: "example.com/repo",
		}, nil
	}

	if pathtools.HasPrefix(importPath, "example.com") {
		return &vcs.RepoRoot{
			VCS:  vcs.ByCmd("git"),
			Repo: "https://example.com",
			Root: "example.com",
		}, nil
	}

	return nil, fmt.Errorf("could not resolve import path: %q", importPath)
}

// stubModInfo is a stub implementation of RemoteCache.ModInfo.
func stubModInfo(importPath string) (string, error) {
	if pathtools.HasPrefix(importPath, "example.com/repo/v2") {
		return "example.com/repo/v2", nil
	}
	if pathtools.HasPrefix(importPath, "example.com/repo") {
		return "example.com/repo", nil
	}
	return "", fmt.Errorf("could not find module for import path: %q", importPath)
}

func convertImportsAttr(r *rule.Rule) interface{} {
	kind := r.Kind()
	value := r.AttrStrings("_imports")
	r.DelAttr("_imports")
	if _, ok := goKinds[kind]; ok {
		return rule.PlatformStrings{Generic: value}
	} else {
		// proto_library
		return value
	}
}

type mapResolver map[string]resolve.Resolver

func (mr mapResolver) Resolver(r *rule.Rule, f string) resolve.Resolver {
	return mr[r.Kind()]
}
