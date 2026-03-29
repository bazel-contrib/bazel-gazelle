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
