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

package plugin

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/google/go-cmp/cmp"
)

const roundTripBuild = `# gazelle:sh_enabled true

load("@rules_shell//shell:sh_library.bzl", "sh_library")

sh_library(
    name = "lib",
    testonly = True,
    srcs = ["lib.sh"],
    data = glob(["*.txt"]),
    env = {
        "A": "1",
        "B": "2",
    },
    shard_count = 2,
    size_delta = -3,
    deps = select({
        "//conditions:default": [":other"],
    }),
)
`

func TestRuleRoundTrip(t *testing.T) {
	f, err := rule.LoadData("BUILD.bazel", "pkg", []byte(roundTripBuild))
	if err != nil {
		t.Fatal(err)
	}
	pf := fileToProtocol(f)
	if diff := cmp.Diff([]protocol.Directive{{Key: "sh_enabled", Value: "true"}}, pf.Directives); diff != "" {
		t.Errorf("directives (-want,+got):\n%s", diff)
	}
	if diff := cmp.Diff([]protocol.Load{{Name: "@rules_shell//shell:sh_library.bzl", Symbols: []string{"sh_library"}}}, pf.Loads); diff != "" {
		t.Errorf("loads (-want,+got):\n%s", diff)
	}
	if len(pf.Rules) != 1 {
		t.Fatalf("got %d rules; want 1", len(pf.Rules))
	}
	pr := pf.Rules[0]
	wantAttrs := map[string]any{
		"srcs":        []any{"lib.sh"},
		"env":         map[string]any{"A": "1", "B": "2"},
		"shard_count": int64(2),
		"size_delta":  int64(-3),
		"testonly":    true,
	}
	if diff := cmp.Diff(wantAttrs, pr.Attrs); diff != "" {
		t.Errorf("attrs (-want,+got):\n%s", diff)
	}
	wantExprs := map[string]string{
		"data": `glob(["*.txt"])`,
		"deps": "select({\n    \"//conditions:default\": [\":other\"],\n})",
	}
	if diff := cmp.Diff(wantExprs, pr.AttrExprs); diff != "" {
		t.Errorf("attr exprs (-want,+got):\n%s", diff)
	}

	// Send the rule over the wire and back, as a plugin that echoes it would.
	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatal(err)
	}
	var decoded protocol.Rule
	if err := protocol.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	r, err := ruleFromProtocol(decoded)
	if err != nil {
		t.Fatal(err)
	}
	out := rule.EmptyFile("BUILD.bazel", "pkg")
	out.Loads = f.Loads
	r.Insert(out)
	got := string(out.Format())
	want := roundTripBuild[strings.Index(roundTripBuild, "sh_library("):]
	if !strings.HasSuffix(got, want) {
		t.Errorf("round trip changed the rule. got:\n%s\nwant:\n%s", got, want)
	}
}

func TestApplyAttrEditErrors(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		edit    protocol.AttrEdit
		wantErr string
	}{
		{
			desc:    "null",
			edit:    protocol.AttrEdit{Attrs: map[string]any{"deps": nil}},
			wantErr: "null is not a valid attribute value",
		},
		{
			desc: "both",
			edit: protocol.AttrEdit{
				Attrs:     map[string]any{"deps": []any{}},
				AttrExprs: map[string]string{"deps": "[]"},
			},
			wantErr: `attribute "deps" set in both attrs and attrExprs`,
		},
		{
			desc:    "bad_expr",
			edit:    protocol.AttrEdit{AttrExprs: map[string]string{"deps": "select("}},
			wantErr: `attribute "deps": parsing expression`,
		},
		{
			desc:    "two_exprs",
			edit:    protocol.AttrEdit{AttrExprs: map[string]string{"deps": "[]\nx = 1"}},
			wantErr: "expected a single expression",
		},
		{
			desc:    "delete_name",
			edit:    protocol.AttrEdit{DeleteAttrs: []string{"name"}},
			wantErr: `attribute "name" can't be deleted`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			r := rule.NewRule("sh_library", "lib")
			err := applyAttrEdit(r, tc.edit)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got error %v; want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestApplyAttrEdit(t *testing.T) {
	r := rule.NewRule("sh_library", "lib")
	r.SetAttr("deps", []string{":old"})
	r.SetAttr("tags", []string{"manual"})
	edit := protocol.AttrEdit{
		Attrs: map[string]any{
			"name": "renamed",
			"srcs": []any{"a.sh", "b.sh"},
			"size": json.Number("7"),
			"meta": map[string]any{"k": []any{"v", json.Number("1")}},
		},
		AttrExprs:   map[string]string{"deps": `select({"//conditions:default": [":new"]})`},
		DeleteAttrs: []string{"tags"},
	}
	if err := applyAttrEdit(r, edit); err != nil {
		t.Fatal(err)
	}
	f := rule.EmptyFile("BUILD.bazel", "")
	r.Insert(f)
	got := strings.TrimSpace(string(f.Format()))
	want := strings.TrimSpace(`
sh_library(
    name = "renamed",
    size = 7,
    srcs = [
        "a.sh",
        "b.sh",
    ],
    meta = {
        "k": [
            "v",
            1,
        ],
    },
    deps = select({"//conditions:default": [":new"]}),
)
`)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want,+got):\n%s", diff)
	}
}

func TestKindFromProtocol(t *testing.T) {
	ki, err := kindFromProtocol(protocol.KindInfo{
		Name:          "sh_library",
		LoadedFrom:    "@rules_shell//shell:sh_library.bzl",
		NonEmptyAttrs: []string{"srcs"},
		ResolveAttrs:  []string{"deps"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ki.LoadedFrom.String() != "@rules_shell//shell:sh_library.bzl" || !ki.NonEmptyAttrs["srcs"] || !ki.ResolveAttrs["deps"] {
		t.Errorf("unexpected kind info: %#v", ki)
	}
	if _, err := kindFromProtocol(protocol.KindInfo{Name: "x", LoadedFrom: ":relative.bzl"}); err == nil {
		t.Error("expected error for relative loadedFrom label")
	}
	if _, err := kindFromProtocol(protocol.KindInfo{}); err == nil {
		t.Error("expected error for kind without name")
	}
}
