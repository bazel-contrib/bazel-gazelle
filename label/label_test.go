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

package label

import (
	"reflect"
	"testing"
)

func TestLabelString(t *testing.T) {
	for _, spec := range []struct {
		l    Label
		want string
	}{
		{
			l:    Label{Name: "foo"},
			want: "//:foo",
		}, {
			l:    Label{Pkg: "foo/bar", Name: "baz"},
			want: "//foo/bar:baz",
		}, {
			l:    Label{Pkg: "foo/bar", Name: "bar"},
			want: "//foo/bar",
		}, {
			l:    Label{Repo: "com_example_repo", Pkg: "foo/bar", Name: "baz"},
			want: "@com_example_repo//foo/bar:baz",
		}, {
			l:    Label{Repo: "com_example_repo", Pkg: "foo/bar", Name: "bar"},
			want: "@com_example_repo//foo/bar",
		}, {
			l:    Label{Relative: true, Name: "foo"},
			want: ":foo",
		}, {
			l:    Label{Repo: "@", Pkg: "foo/bar", Name: "baz"},
			want: "@//foo/bar:baz",
		}, {
			l:    Label{Repo: "@", Pkg: "foo/bar", Name: "baz"},
			want: "@//foo/bar:baz",
		},
	} {
		if got, want := spec.l.String(), spec.want; got != want {
			t.Errorf("%#v.String() = %q; want %q", spec.l, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		str     string
		want    Label
		wantErr bool
	}{
		{str: "", wantErr: true},
		{str: "@//:", wantErr: true},
		{str: "@a:b", wantErr: true},
		{str: "@a//", wantErr: true},
		{str: "@//:a", want: Label{Repo: "@", Name: "a", Relative: false}},
		{str: "@//a:b", want: Label{Repo: "@", Pkg: "a", Name: "b"}},
		{str: ":a", want: Label{Name: "a", Relative: true}},
		{str: "a", want: Label{Name: "a", Relative: true}},
		{str: "//:a", want: Label{Name: "a", Relative: false}},
		{str: "//a", want: Label{Pkg: "a", Name: "a"}},
		{str: "//a/b", want: Label{Pkg: "a/b", Name: "b"}},
		{str: "//a:b", want: Label{Pkg: "a", Name: "b"}},
		{str: "@a", want: Label{Repo: "a", Pkg: "", Name: "a"}},
		{str: "@a//b", want: Label{Repo: "a", Pkg: "b", Name: "b"}},
		{str: "@a//b:c", want: Label{Repo: "a", Pkg: "b", Name: "c"}},
		{str: "@a//@b:c", want: Label{Repo: "a", Pkg: "@b", Name: "c"}},
		{str: "@..//b:c", want: Label{Repo: "..", Pkg: "b", Name: "c"}},
		{str: "@--//b:c", want: Label{Repo: "--", Pkg: "b", Name: "c"}},
		{str: "//api_proto:api.gen.pb.go_checkshtest", want: Label{Pkg: "api_proto", Name: "api.gen.pb.go_checkshtest"}},
		{str: "@go_sdk//:src/cmd/go/testdata/mod/rsc.io_!q!u!o!t!e_v1.5.2.txt", want: Label{Repo: "go_sdk", Name: "src/cmd/go/testdata/mod/rsc.io_!q!u!o!t!e_v1.5.2.txt"}},
		{str: "//:a][b", want: Label{Name: "a][b"}},
		{str: "//:a b", want: Label{Name: "a b"}},
		{str: "//some/pkg/[someId]:someId", want: Label{Pkg: "some/pkg/[someId]", Name: "someId"}},
		{str: "//some/pkg/[someId]:[someId]", want: Label{Pkg: "some/pkg/[someId]", Name: "[someId]"}},
		{str: "@a//some/pkg/[someId]:[someId]", want: Label{Repo: "a", Pkg: "some/pkg/[someId]", Name: "[someId]"}},
		{str: "@rules_python~0.0.0~pip~name_dep//:_pkg", want: Label{Repo: "rules_python~0.0.0~pip~name_dep", Name: "_pkg"}},
		{str: "@rules_python~0.0.0~pip~name//:dep_pkg", want: Label{Repo: "rules_python~0.0.0~pip~name", Name: "dep_pkg"}},
		{str: "@@rules_python~0.26.0~python~python_3_10_x86_64-unknown-linux-gnu//:python_runtimes", want: Label{Repo: "rules_python~0.26.0~python~python_3_10_x86_64-unknown-linux-gnu", Name: "python_runtimes", Canonical: true}},
		{str: "@rules_python++pip+name_dep//:_pkg", want: Label{Repo: "rules_python++pip+name_dep", Name: "_pkg"}},
		{str: "@rules_python++pip+name//:dep_pkg", want: Label{Repo: "rules_python++pip+name", Name: "dep_pkg"}},
		{str: "@@rules_python++python+python_3_10_x86_64-unknown-linux-gnu//:python_runtimes", want: Label{Repo: "rules_python++python+python_3_10_x86_64-unknown-linux-gnu", Name: "python_runtimes", Canonical: true}},
		{str: "@@+toolchains+jfrog_linux_amd64//:jfrog_toolchain", want: Label{Repo: "+toolchains+jfrog_linux_amd64", Name: "jfrog_toolchain", Canonical: true}},
	} {
		got, err := Parse(tc.str)
		if err != nil && !tc.wantErr {
			t.Errorf("for string %q: got error %s ; want success", tc.str, err)
			continue
		}
		if err == nil && tc.wantErr {
			t.Errorf("for string %q: got label %s ; want error", tc.str, got)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("for string %q: got %s ; want %s", tc.str, got, tc.want)
		}
	}
}

func TestImportPathToBazelRepoName(t *testing.T) {
	for path, want := range map[string]string{
		"git.sr.ht/~urandom/errors": "ht_sr_git_urandom_errors",
		"golang.org/x/mod":          "org_golang_x_mod",
	} {
		if got := ImportPathToBazelRepoName(path); got != want {
			t.Errorf(`ImportPathToBazelRepoName(%q) = %q; want %q`, path, got, want)
		}
	}
}

func TestParseStringRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		in  string
		out string
	}{
		{in: "target", out: ":target"},
		{in: ":target"},
		{in: "//:target"},
		{in: "//pkg:target"},
		{in: "@repo//:target"},
		{in: "@repo//pkg:target"},
		{in: "@repo", out: "@repo//:repo"},
		{in: "@//pkg:target"},
		{in: "@@canonical~name//:target"},
		{in: "@@//:target"},
	} {
		lbl, err := Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", tc, err)
			continue
		}
		got := lbl.String()
		var want string
		if len(tc.out) == 0 {
			want = tc.in
		} else {
			want = tc.out
		}
		if got != want {
			t.Errorf("Parse(%q).String() = %q; want %q", tc.in, got, want)
		}
	}
}
