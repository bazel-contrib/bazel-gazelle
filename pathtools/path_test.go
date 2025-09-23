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

func TestIndex(t *testing.T) {
	for _, tc := range []struct {
		desc, p, sub string
		want         int
	}{
		{
			desc: "empty",
			p:    "",
			sub:  "",
			want: 0,
		},
		{
			desc: "empty_p",
			p:    "",
			sub:  "a",
			want: -1,
		},
		{
			desc: "empty_sub",
			p:    "a",
			sub:  "",
			want: 0,
		},
		{
			desc: "match_start_1",
			p:    "a/b/c",
			sub:  "a",
			want: 0,
		},
		{
			desc: "match_start_2",
			p:    "aa/bb/cc",
			sub:  "aa/bb",
			want: 0,
		},
		{
			desc: "match_first",
			p:    "aa/aa",
			sub:  "aa",
			want: 0,
		},
		{
			desc: "match_full",
			p:    "aaa/bbb/ccc",
			sub:  "aaa/bbb/ccc",
			want: 0,
		},
		{
			desc: "match_middle_2",
			p:    "a/b/c/d",
			sub:  "b/c",
			want: 2,
		},
		{
			desc: "match_end_2",
			p:    "aa/bb/cc",
			sub:  "bb/cc",
			want: 3,
		},
		{
			desc: "match_end_1",
			p:    "a/b/c",
			sub:  "c",
			want: 4,
		},
		{
			desc: "partial_match_start",
			p:    "aa/bb",
			sub:  "aa/b",
			want: -1,
		},
		{
			desc: "partial_match_end",
			p:    "aa/bb",
			sub:  "a/bb",
			want: -1,
		},
		{
			desc: "match_abs_both_start",
			p:    "/a/b",
			sub:  "/a",
			want: 0,
		},
		{
			desc: "match_abs_p_start",
			p:    "/a/b",
			sub:  "a",
			want: 1,
		},
		{
			desc: "match_abs_sub_start",
			p:    "a/b",
			sub:  "/a",
			want: -1,
		},
		{
			desc: "partial_match_abs",
			p:    "/aa/bb",
			sub:  "/aa/b",
			want: -1,
		},
		{
			desc: "match_unclean_dots",
			p:    "a/b/../c",
			sub:  "b/..",
			want: 2,
		},
		{
			desc: "match_unclean_slashes",
			p:    "a/b//c",
			sub:  "b//c",
			want: 2,
		},
		{
			desc: "match_unclean_slashes_no",
			p:    "a/b//c",
			sub:  "b/c",
			want: -1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if got := Index(tc.p, tc.sub); got != tc.want {
				t.Errorf("got %d; want %d", got, tc.want)
			}
		})
	}
}

func TestLastIndex(t *testing.T) {
	for _, tc := range []struct {
		desc, p, sub string
		want         int
	}{
		{
			desc: "empty",
			p:    "",
			sub:  "",
			want: 0,
		},
		{
			desc: "empty_p",
			p:    "",
			sub:  "a",
			want: -1,
		},
		{
			desc: "empty_sub",
			p:    "a",
			sub:  "",
			want: 1,
		},
		{
			desc: "match_start_1",
			p:    "a/b/c",
			sub:  "a",
			want: 0,
		},
		{
			desc: "match_start_2",
			p:    "aa/bb/cc",
			sub:  "aa/bb",
			want: 0,
		},
		{
			desc: "match_last",
			p:    "aa/aa",
			sub:  "aa",
			want: 3,
		},
		{
			desc: "match_full",
			p:    "aaa/bbb/ccc",
			sub:  "aaa/bbb/ccc",
			want: 0,
		},
		{
			desc: "match_middle_2",
			p:    "a/b/c/d",
			sub:  "b/c",
			want: 2,
		},
		{
			desc: "match_end_2",
			p:    "aa/bb/cc",
			sub:  "bb/cc",
			want: 3,
		},
		{
			desc: "match_end_1",
			p:    "a/b/c",
			sub:  "c",
			want: 4,
		},
		{
			desc: "partial_match_start",
			p:    "aa/bb",
			sub:  "aa/b",
			want: -1,
		},
		{
			desc: "partial_match_end",
			p:    "aa/bb",
			sub:  "a/bb",
			want: -1,
		},
		{
			desc: "match_abs_both_start",
			p:    "/a/b",
			sub:  "/a",
			want: 0,
		},
		{
			desc: "match_abs_p_start",
			p:    "/a/b",
			sub:  "a",
			want: 1,
		},
		{
			desc: "match_abs_sub_start",
			p:    "a/b",
			sub:  "/a",
			want: -1,
		},
		{
			desc: "partial_match_abs",
			p:    "/aa/bb",
			sub:  "/aa/b",
			want: -1,
		},
		{
			desc: "match_unclean_dots",
			p:    "a/b/../c",
			sub:  "b/..",
			want: 2,
		},
		{
			desc: "match_unclean_slashes",
			p:    "a/b//c",
			sub:  "b//c",
			want: 2,
		},
		{
			desc: "match_unclean_slashes_no",
			p:    "a/b//c",
			sub:  "b/c",
			want: -1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if got := LastIndex(tc.p, tc.sub); got != tc.want {
				t.Errorf("got %d; want %d", got, tc.want)
			}
		})
	}
}

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
