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

package pathtools

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHasPrefix(t *testing.T) {
	for _, tc := range []struct {
		desc, path, prefix string
		want               bool
	}{
		{
			desc:   "empty_prefix",
			path:   "home/jr_hacker",
			prefix: "",
			want:   true,
		}, {
			desc:   "partial_prefix",
			path:   "home/jr_hacker",
			prefix: "home",
			want:   true,
		}, {
			desc:   "full_prefix",
			path:   "home/jr_hacker",
			prefix: "home/jr_hacker",
			want:   true,
		}, {
			desc:   "too_long",
			path:   "home",
			prefix: "home/jr_hacker",
			want:   false,
		}, {
			desc:   "partial_component",
			path:   "home/jr_hacker",
			prefix: "home/jr_",
			want:   false,
		}, {
			desc:   "trailing_slash_prefix",
			path:   "home/jr_hacker",
			prefix: "home/",
			want:   true,
		}, {
			desc:   "trailing_slash_path",
			path:   "home/jr_hacker/",
			prefix: "home",
			want:   true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if got := HasPrefix(tc.path, tc.prefix); got != tc.want {
				t.Errorf("got %v ; want %v", got, tc.want)
			}
		})
	}
}

func TestTrimPrefix(t *testing.T) {
	for _, tc := range []struct {
		desc, path, prefix, want string
	}{
		{
			desc:   "empty_prefix",
			path:   "home/jr_hacker",
			prefix: "",
			want:   "home/jr_hacker",
		}, {
			desc:   "partial_prefix",
			path:   "home/jr_hacker",
			prefix: "home",
			want:   "jr_hacker",
		}, {
			desc:   "full_prefix",
			path:   "home/jr_hacker",
			prefix: "home/jr_hacker",
			want:   "",
		}, {
			desc:   "partial_component",
			path:   "home/jr_hacker",
			prefix: "home/jr_",
			want:   "home/jr_hacker",
		}, {
			desc:   "trailing_slash_prefix",
			path:   "home/jr_hacker",
			prefix: "home/",
			want:   "jr_hacker",
		}, {
			desc:   "trailing_slash_path",
			path:   "home/jr_hacker/",
			prefix: "home",
			want:   "jr_hacker",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if got := TrimPrefix(tc.path, tc.prefix); got != tc.want {
				t.Errorf("got %q ; want %q", got, tc.want)
			}
		})
	}
}

func TestPrefixes(t *testing.T) {
	for _, test := range []struct {
		name, path string
		want       []string
	}{
		{
			name: "empty",
			path: "",
			want: []string{""},
		},
		{
			name: "slash",
			path: "/",
			want: []string{"/"},
		},
		{
			name: "simple_rel",
			path: "a",
			want: []string{"", "a"},
		},
		{
			name: "simple_abs",
			path: "/a",
			want: []string{"/", "/a"},
		},
		{
			name: "multiple_rel",
			path: "aa/bb/cc",
			want: []string{"", "aa", "aa/bb", "aa/bb/cc"},
		},
		{
			name: "multiple_abs",
			path: "/aa/bb/cc",
			want: []string{"/", "/aa", "/aa/bb", "/aa/bb/cc"},
		},
		{
			name: "unclean",
			path: "a/../b//c/",
			want: []string{"", "a", "a/..", "a/../b", "a/../b//c"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			Prefixes(test.path)(func(prefix string) bool {
				got = append(got, prefix)
				return true
			})
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("bad prefixes (-want, +got): %s", diff)
			}
		})
	}
}
