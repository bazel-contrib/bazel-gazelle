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

package rule

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	bzl "github.com/bazelbuild/buildtools/build"
)

func TestParseDirectives(t *testing.T) {
	for _, tc := range []struct {
		desc, content string
		want          []Directive
	}{
		{
			desc: "empty file",
		}, {
			desc: "locations",
			content: `# gazelle:ignore top

#gazelle:ignore before
foo(
   "foo",  # gazelle:ignore inside
) # gazelle:ignore suffix
#gazelle:ignore after

# gazelle:ignore bottom`,
			want: []Directive{
				{"ignore", "top"},
				{"ignore", "before"},
				{"ignore", "after"},
				{"ignore", "bottom"},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := bzl.Parse("test.bazel", []byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}

			got := parseDirectives(f.Stmt)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v ; want %#v", got, tc.want)
			}
		})
	}
}

func TestParseDirectivesFromFile(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		content string
		want    []Directive
		wantErr bool
	}{
		{
			desc:    "empty file",
			content: "",
			want:    nil,
		},
		{
			desc: "directives with hash prefix",
			content: `# gazelle:resolve go example.com/foo //third_party:foo
# gazelle:resolve proto google/api/annotations.proto @go_googleapis//google/api:annotations_proto
`,
			want: []Directive{
				{"resolve", "go example.com/foo //third_party:foo"},
				{"resolve", "proto google/api/annotations.proto @go_googleapis//google/api:annotations_proto"},
			},
		},
		{
			desc: "blank lines and plain comments ignored",
			content: `# This is a plain comment
# gazelle:resolve go example.com/foo //third_party:foo

# Another comment
# gazelle:exclude vendor
`,
			want: []Directive{
				{"resolve", "go example.com/foo //third_party:foo"},
				{"exclude", "vendor"},
			},
		},
		{
			desc: "no space after hash",
			content: `#gazelle:resolve go example.com/foo //foo
`,
			want: []Directive{
				{"resolve", "go example.com/foo //foo"},
			},
		},
		{
			desc: "mixed directive types",
			content: `# gazelle:resolve go example.com/foo //third_party:foo
# gazelle:resolve_regexp go ^example.com/bar/(.*)$ //bar/${1}
# gazelle:exclude vendor
# gazelle:ignore
`,
			want: []Directive{
				{"resolve", "go example.com/foo //third_party:foo"},
				{"resolve_regexp", "go ^example.com/bar/(.*)$ //bar/${1}"},
				{"exclude", "vendor"},
				{"ignore", ""},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, "directives.cfg")
			if err := os.WriteFile(filePath, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := ParseDirectivesFromFile(filePath)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v ; want %#v", got, tc.want)
			}
		})
	}
}

func TestParseDirectivesFromFile_FileNotFound(t *testing.T) {
	_, err := ParseDirectivesFromFile("/nonexistent/file.cfg")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}
