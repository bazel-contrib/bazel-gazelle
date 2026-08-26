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

package golang

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/bazel-gazelle/walk"
)

func TestAncestorUpdates(t *testing.T) {
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "embed/BUILD.bazel"},
		{Path: "embed/embed.go", Content: "package embed\n\nimport _ \"embed\"\n\n//go:embed assets/**\nvar assets string\n"},
		{Path: "embed/assets/deep/new.json", Content: "{}"},
		{Path: "plain/BUILD.bazel"},
		{Path: "plain/plain.go", Content: "package plain\n"},
		{Path: "plain/assets/deep/new.json", Content: "{}"},
		{Path: "test/BUILD.bazel"},
		{Path: "test/test.go", Content: "package test\n"},
		{Path: "test/testdata/deep/case.txt", Content: "case"},
		{Path: "bounded/BUILD.bazel"},
		{Path: "bounded/embed.go", Content: "package bounded\n\nimport _ \"embed\"\n\n//go:embed child/**\nvar child string\n"},
		{Path: "bounded/child/BUILD.bazel"},
		{Path: "bounded/child/child.go", Content: "package child\n"},
		{Path: "bounded/child/deep/file.txt", Content: "file"},
	})
	defer cleanup()

	tests := []struct {
		name string
		rel  string
		want []string
	}{
		{name: "embed", rel: "embed/assets/deep", want: []string{"embed/assets/deep", "embed"}},
		{name: "no embed", rel: "plain/assets/deep", want: []string{"plain/assets/deep"}},
		{name: "testdata", rel: "test/testdata/deep", want: []string{"test/testdata/deep", "test"}},
		{name: "build boundary itself", rel: "bounded/child", want: []string{"bounded/child", "bounded"}},
		{name: "below build boundary", rel: "bounded/child/deep", want: []string{"bounded/child/deep"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, langs, cexts := testConfig(t, "-repo_root="+dir, "-go_prefix=example.com")
			gl := langs[1].(*goLang)

			var updatedRels []string
			err := walk.Walk2(c, cexts, []string{filepath.Join(dir, filepath.FromSlash(tt.rel))}, walk.UpdateDirsMode, func(args walk.Walk2FuncArgs) walk.Walk2FuncResult {
				if !args.Update {
					return walk.Walk2FuncResult{}
				}
				updatedRels = append(updatedRels, args.Rel)
				res := gl.ancestorUpdates(args.Rel)
				return walk.Walk2FuncResult{RelsToUpdate: res}
			})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, updatedRels); diff != "" {
				t.Errorf("updated directories (-want,+got):\n%s", diff)
			}
		})
	}
}
