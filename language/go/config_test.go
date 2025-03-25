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
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/bazel-gazelle/walk"
	"github.com/google/go-cmp/cmp"
)

func testConfig(t *testing.T, args ...string) (*config.Config, []language.Language, []config.Configurer) {
	// Add a -repo_root argument if none is present. Without this,
	// config.CommonConfigurer will try to auto-detect a WORKSPACE file,
	// which will fail.
	haveRoot := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-repo_root") {
			haveRoot = true
			break
		}
	}
	if !haveRoot {
		args = append(args, "-repo_root=.")
	}

	cexts := []config.Configurer{
		&config.CommonConfigurer{},
		&walk.Configurer{},
		&resolve.Configurer{},
	}
	langs := []language.Language{proto.NewLanguage(), NewLanguage()}
	c := testtools.NewTestConfig(t, cexts, langs, args)
	for _, lang := range langs {
		cexts = append(cexts, lang)
	}
	return c, langs, cexts
}

func TestCommandLine(t *testing.T) {
	c, _, _ := testConfig(
		t,
		"-build_tags=foo,bar",
		"-go_prefix=example.com/repo",
		"-go_naming_convention=import_alias",
		"-external=vendored",
		"-repo_root=.")
	gc := getGoConfig(c)
	for _, tag := range []string{"foo", "bar", "gc"} {
		if !gc.genericTags[tag] {
			t.Errorf("expected tag %q to be set", tag)
		}
	}
	if gc.prefix != "example.com/repo" {
		t.Errorf(`got prefix %q; want "example.com/repo"`, gc.prefix)
	}
	if gc.depMode != vendorMode {
		t.Errorf("got dep mode %v; want %v", gc.depMode, vendorMode)
	}
	if gc.goNamingConvention != importAliasNamingConvention {
		t.Errorf("got naming convention %v; want %v", gc.goNamingConvention, importAliasNamingConvention)
	}
}

func TestDirectives(t *testing.T) {
	c, _, cexts := testConfig(t)
	content := []byte(`
# gazelle:build_tags foo,bar
# gazelle:importmap_prefix x
# gazelle:prefix y
# gazelle:go_grpc_compilers abc, def
# gazelle:go_proto_compilers foo, bar
`)
	f, err := rule.LoadData(filepath.FromSlash("test/BUILD.bazel"), "test", content)
	if err != nil {
		t.Fatal(err)
	}
	for _, cext := range cexts {
		cext.Configure(c, "test", f)
	}
	gc := getGoConfig(c)
	for _, tag := range []string{"foo", "bar", "gc"} {
		if !gc.genericTags[tag] {
			t.Errorf("expected tag %q to be set", tag)
		}
	}
	if gc.prefix != "y" {
		t.Errorf(`got prefix %q; want "y"`, gc.prefix)
	}
	if gc.prefixRel != "test" {
		t.Errorf(`got prefixRel %q; want "test"`, gc.prefixRel)
	}
	if gc.importMapPrefix != "x" {
		t.Errorf(`got importmapPrefix %q; want "x"`, gc.importMapPrefix)
	}
	if gc.importMapPrefixRel != "test" {
		t.Errorf(`got importmapPrefixRel %q; want "test"`, gc.importMapPrefixRel)
	}
	if !gc.goGrpcCompilersSet {
		t.Error("expected goGrpcCompilersSet to be set")
	}
	if diff := cmp.Diff([]string{"abc", "def"}, gc.goGrpcCompilers); diff != "" {
		t.Errorf("(-want, +got): %s", diff)
	}

	if !gc.goProtoCompilersSet {
		t.Error("expected goProtoCompilersSet to be set")
	}
	if diff := cmp.Diff(gc.goProtoCompilers, []string{"foo", "bar"}); diff != "" {
		t.Errorf("(-want, +got): %s", diff)
	}

	subContent := []byte(`
# gazelle:go_grpc_compilers
# gazelle:go_proto_compilers
`)
	f, err = rule.LoadData(filepath.FromSlash("test/sub/BUILD.bazel"), "sub", subContent)
	if err != nil {
		t.Fatal(err)
	}
	for _, cext := range cexts {
		cext.Configure(c, "test/sub", f)
	}
	gc = getGoConfig(c)
	if gc.goGrpcCompilersSet {
		t.Error("expected goGrpcCompilersSet to be unset")
	}
	if diff := cmp.Diff(defaultGoGrpcCompilers, gc.goGrpcCompilers); diff != "" {
		t.Errorf("(-want, +got): %s", diff)
	}

	if gc.goProtoCompilersSet {
		t.Error("expected goProtoCompilersSet to be unset")
	}
	if diff := cmp.Diff(defaultGoProtoCompilers, gc.goProtoCompilers); diff != "" {
		t.Errorf("(-want, +got): %s", diff)
	}

}

func TestVendorConfig(t *testing.T) {
	c, _, cexts := testConfig(t)
	gc := getGoConfig(c)
	gc.prefix = "example.com/repo"
	gc.prefixRel = ""
	gc.importMapPrefix = "bad-importmap-prefix"
	gc.importMapPrefixRel = ""
	for _, cext := range cexts {
		cext.Configure(c, "x/vendor", nil)
	}
	gc = getGoConfig(c)
	if gc.prefix != "" {
		t.Errorf(`prefix: got %q; want ""`, gc.prefix)
	}
	if gc.prefixRel != "x/vendor" {
		t.Errorf(`prefixRel: got %q; want "x/vendor"`, gc.prefixRel)
	}
	if gc.importMapPrefix != "example.com/repo/x/vendor" {
		t.Errorf(`importMapPrefix: got %q; want "example.com/repo/x/vendor"`, gc.importMapPrefix)
	}
	if gc.importMapPrefixRel != "x/vendor" {
		t.Errorf(`importMapPrefixRel: got %q; want "x/vendor"`, gc.importMapPrefixRel)
	}
}

func TestInferProtoMode(t *testing.T) {
	c, _, cexts := testConfig(t)
	for _, tc := range []struct {
		desc, rel, content string
		old                proto.Mode
		explicit           bool
		want               proto.Mode
	}{
		{
			desc: "default_empty",
			old:  proto.DefaultMode,
			want: proto.DefaultMode,
		}, {
			desc: "default_to_legacy",
			old:  proto.DefaultMode,
			content: `
load("@io_bazel_rules_go//proto:go_proto_library.bzl", "go_proto_library")

go_proto_library(
    name = "foo_proto",
)
`,
			want: proto.LegacyMode,
		}, {
			desc: "default_to_disable",
			old:  proto.DefaultMode,
			content: `
load("@some_repo//:custom.bzl", "go_proto_library")
`,
			want: proto.DisableMode,
		}, {
			desc: "vendor_disable",
			old:  proto.DefaultMode,
			rel:  "vendor",
			want: proto.DisableMode,
		}, {
			desc:     "explicit_override_legacy",
			old:      proto.DefaultMode,
			explicit: true,
			content: `
load("@io_bazel_rules_go//proto:go_proto_library.bzl", "go_proto_library")
`,
			want: proto.DefaultMode,
		}, {
			desc:     "explicit_override_vendor",
			old:      proto.DefaultMode,
			explicit: true,
			rel:      "vendor",
			want:     proto.DefaultMode,
		}, {
			desc:     "disable_override_legacy",
			old:      proto.DisableMode,
			explicit: false,
			content: `
load("@io_bazel_rules_go//proto:go_proto_library.bzl", "go_proto_library")
`,
			want: proto.DisableMode,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c := c.Clone()
			pc := proto.GetProtoConfig(c)
			pc.Mode = tc.old
			pc.ModeExplicit = tc.explicit
			var f *rule.File
			if tc.content != "" {
				var err error
				f, err = rule.LoadData(path.Join(tc.rel, "BUILD.bazel"), tc.rel, []byte(tc.content))
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, cext := range cexts {
				cext.Configure(c, tc.rel, f)
			}
			pc = proto.GetProtoConfig(c)
			if pc.Mode != tc.want {
				t.Errorf("got %v; want %v", pc.Mode, tc.want)
			}
		})
	}
}

func TestPreprocessTags(t *testing.T) {
	gc := newGoConfig()
	expectedTags := []string{"gc"}
	for _, tag := range expectedTags {
		if !gc.genericTags[tag] {
			t.Errorf("tag %q not set", tag)
		}
	}
	unexpectedTags := []string{"x", "cgo", "go1.8", "go1.7"}
	for _, tag := range unexpectedTags {
		if gc.genericTags[tag] {
			t.Errorf("tag %q unexpectedly set", tag)
		}
	}
}

func TestPrefixFallback(t *testing.T) {
	c, _, cexts := testConfig(t)
	for _, tc := range []struct {
		desc, content, want string
	}{
		{
			desc: "go_prefix",
			content: `
go_prefix("example.com/repo")
`,
			want: "example.com/repo",
		}, {
			desc: "gazelle",
			content: `
gazelle(
    name = "gazelle",
    prefix = "example.com/repo",
)
`,
			want: "example.com/repo",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := rule.LoadData("BUILD.bazel", "", []byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			for _, cext := range cexts {
				cext.Configure(c, "x", f)
			}
			gc := getGoConfig(c)
			if !gc.prefixSet {
				t.Fatalf("prefix not set")
			}
			if gc.prefix != tc.want {
				t.Errorf("prefix: want %q; got %q", gc.prefix, tc.want)
			}
			if gc.prefixRel != "x" {
				t.Errorf("rel: got %q; want %q", gc.prefixRel, "x")
			}
		})
	}
}

func TestSplitValue(t *testing.T) {
	for _, tc := range []struct {
		value string
		parts []string
	}{
		{
			value: "\t foo, bar \t",
			parts: []string{"foo", "bar"},
		},
		{
			value: " foo ",
			parts: []string{"foo"},
		},
	} {
		parts := splitValue(tc.value)
		if diff := cmp.Diff(tc.parts, parts); diff != "" {
			t.Errorf("(-want, +got): %s", diff)
		}

	}
}
