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

package proto

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

func TestResolveProto(t *testing.T) {
	type buildFile struct {
		rel, content string
	}
	type testCase struct {
		desc      string
		index     []buildFile
		old, want string
	}
	for _, tc := range []testCase{
		{
			desc: "well_known",
			index: []buildFile{{
				rel: "google/protobuf",
				content: `
proto_library(
    name = "bad_proto",
    srcs = ["any.proto"],
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = [
        "google/protobuf/any.proto",
        "google/protobuf/timestamp.proto",
    ],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = [
        "@com_google_protobuf//:any_proto",
        "@com_google_protobuf//:timestamp_proto",
    ],
)
`,
		}, {
			desc: "override",
			index: []buildFile{
				{
					rel: "google/rpc",
					content: `
proto_library(
    name = "bad_proto",
    srcs = ["status.proto"],
)
`,
				}, {
					rel: "",
					content: `
# gazelle:resolve proto google/rpc/status.proto //:good_proto
`,
				},
			},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["google/rpc/status.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//:good_proto"],
)
`,
		}, {
			desc: "index",
			index: []buildFile{{
				rel: "foo",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["foo/foo.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//foo:foo_proto"],
)
`,
		}, {
			desc: "index_local",
			old: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

proto_library(
    name = "dep_proto",
    _imports = ["test/foo.proto"],
)
`,
			want: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

proto_library(
    name = "dep_proto",
    deps = [":foo_proto"],
)
`,
		}, {
			desc: "index_ambiguous",
			index: []buildFile{{
				rel: "foo",
				content: `
proto_library(
    name = "a_proto",
    srcs = ["foo.proto"],
)

proto_library(
    name = "b_proto",
    srcs = ["foo.proto"],
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["foo/foo.proto"],
)
`,
			want: `proto_library(name = "dep_proto")`,
		}, {
			desc: "index_self",
			old: `
proto_library(
    name = "dep_proto",
    srcs = ["foo.proto"],
    _imports = ["test/foo.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    srcs = ["foo.proto"],
)
`,
		}, {
			desc: "index_dedup",
			index: []buildFile{{
				rel: "foo",
				content: `
proto_library(
    name = "foo_proto",
    srcs = [
        "a.proto",
        "b.proto",
    ],
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    srcs = ["dep.proto"],
    _imports = [
        "foo/a.proto",
        "foo/b.proto",
    ],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    srcs = ["dep.proto"],
    deps = ["//foo:foo_proto"],
)
`,
		}, {
			desc: "unknown",
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["foo/bar/unknown.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//foo/bar:bar_proto"],
)
`,
		}, {
			desc: "strip_import_prefix",
			index: []buildFile{{
				rel: "foo/bar/sub",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    strip_import_prefix = "/foo/bar",
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["sub/foo.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//foo/bar/sub:foo_proto"],
)
`,
		}, {
			desc: "skip bad strip_import_prefix",
			index: []buildFile{{
				rel: "bar",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    strip_import_prefix = "/foo",
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["bar/foo.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//bar:bar_proto"],
)
`,
		}, {
			desc: "import_prefix",
			index: []buildFile{{
				rel: "bar",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    import_prefix = "foo/",
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["foo/bar/foo.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//bar:foo_proto"],
)
`,
		}, {
			desc: "strip_import_prefix and import_prefix",
			index: []buildFile{{
				rel: "foo",
				content: `
proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
    import_prefix = "bar/",
    strip_import_prefix = "/foo",
)
`,
			}},
			old: `
proto_library(
    name = "dep_proto",
    _imports = ["bar/foo.proto"],
)
`,
			want: `
proto_library(
    name = "dep_proto",
    deps = ["//foo:foo_proto"],
)
`,
		}, {
			desc: "test single file resolution in file mode",
			index: []buildFile{{
				rel: "somedir",
				content: `
# gazelle:proto file

proto_library(
    name = "foo_proto",
    srcs = ["foo.proto"],
)

proto_library(
    name = "bar_proto",
    srcs = ["bar.proto"],
)

proto_library(
    name = "baz_proto",
    srcs = ["baz.proto"],
)
`,
			}},
			old: `
proto_library(
    name = "other_proto",
    _imports = ["somedir/bar.proto"],
)
`,
			want: `
proto_library(
    name = "other_proto",
    deps = ["//somedir:bar_proto"],
)
`,
		}, {
			desc: "test single file resolution in same package",
			old: `
proto_library(
    name = "qwerty_proto",
    srcs = ["qwerty.proto"],
)

proto_library(
    name = "other_proto",
    _imports = ["test/qwerty.proto"],
)
`,
			want: `
proto_library(
    name = "qwerty_proto",
    srcs = ["qwerty.proto"],
)

proto_library(
    name = "other_proto",
    deps = [":qwerty_proto"],
)
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c, lang, cexts := testConfig(t, ".")
			mrslv := make(mapResolver)
			mrslv["proto_library"] = lang
			ix := resolve.NewRuleIndex(mrslv.Resolver, []resolve.CrossResolver{lang.(resolve.CrossResolver)})
			rc := (*repo.RemoteCache)(nil)
			for _, bf := range tc.index {
				f, err := rule.LoadData(filepath.Join(bf.rel, "BUILD.bazel"), bf.rel, []byte(bf.content))
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
			f, err := rule.LoadData("test/BUILD.bazel", "test", []byte(tc.old))
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
				lang.Resolve(c, ix, rc, r, imports[i], label.New("", "test", r.Name()))
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

func TestCrossResolve(t *testing.T) {
	type testCase struct {
		desc      string
		protoMode Mode
		imp       resolve.ImportSpec
		lang      string
		want      []resolve.FindResult
	}
	for _, tc := range []testCase{
		{
			desc:      "disable global mode go",
			protoMode: DisableGlobalMode,
			imp:       resolve.ImportSpec{Lang: "go", Imp: "github.com/golang/protobuf/proto"},
			lang:      "go",
			want:      nil,
		},
		{
			desc:      "disable global mode proto",
			protoMode: DisableGlobalMode,
			imp:       resolve.ImportSpec{Lang: "proto", Imp: "google/protobuf/any.proto"},
			lang:      "go",
			want:      nil,
		},
		{
			desc:      "proto source lang",
			protoMode: DefaultMode,
			imp:       resolve.ImportSpec{Lang: "proto", Imp: "google/protobuf/any.proto"},
			lang:      "proto",
			want:      nil,
		},
		{
			desc:      "unsupported import lang",
			protoMode: DefaultMode,
			imp:       resolve.ImportSpec{Lang: "foo", Imp: "foo"},
			lang:      "go",
			want:      nil,
		},
		{
			desc:      "go unknown import",
			protoMode: DefaultMode,
			imp:       resolve.ImportSpec{Lang: "go", Imp: "foo"},
			lang:      "go",
			want:      nil,
		},
		{
			desc:      "proto known import",
			protoMode: DefaultMode,
			imp:       resolve.ImportSpec{Lang: "proto", Imp: "google/protobuf/any.proto"},
			lang:      "go",
			want:      []resolve.FindResult{{Label: label.New("com_github_golang_protobuf", "ptypes/any", "any")}},
		},
		{
			desc:      "proto unknown import",
			protoMode: DefaultMode,
			imp:       resolve.ImportSpec{Lang: "proto", Imp: "foo.proto"},
			lang:      "go",
			want:      nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c, lang, _ := testConfig(t, ".")
			pc := GetProtoConfig(c)
			pc.Mode = tc.protoMode
			ix := (*resolve.RuleIndex)(nil)
			got := lang.(resolve.CrossResolver).CrossResolve(c, ix, tc.imp, tc.lang)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v ; want %#v", got, tc.want)
			}
		})
	}
}

func convertImportsAttr(r *rule.Rule) interface{} {
	value := r.AttrStrings("_imports")
	if value == nil {
		value = []string(nil)
	}
	r.DelAttr("_imports")
	return value
}

type mapResolver map[string]resolve.Resolver

func (mr mapResolver) Resolver(r *rule.Rule, f string) resolve.Resolver {
	return mr[r.Kind()]
}
