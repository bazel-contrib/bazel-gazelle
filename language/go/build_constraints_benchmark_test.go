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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var benchmarkBuildTags *buildTags

func BenchmarkReadTags(b *testing.B) {
	for _, tc := range []struct {
		name, source string
	}{
		{
			name:   "no_build_tags",
			source: strings.Repeat("// Copyright holder\n", 100) + "\npackage foo\n",
		},
		{
			name:   "plus_build",
			source: "// +build linux darwin\n\npackage foo\n",
		},
		{
			name:   "large_header",
			source: strings.Repeat("// Copyright holder\n", 1000) + "\npackage foo\n",
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "foo.go")
			if err := os.WriteFile(path, []byte(tc.source), 0o600); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				var err error
				benchmarkBuildTags, err = readTags(path)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
