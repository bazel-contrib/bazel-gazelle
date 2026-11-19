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

package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/graph"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/google/go-cmp/cmp"
)

// depLang is a toy extension for testing the build graph. In each directory
// with a file named "lib.dep", it generates dep_library(name = <dir base>).
// Each line of lib.dep is an import of another directory's path. The
// library is importable by its directory path.
//
// depLang resolves dependencies the legacy way (Resolver). graphDepLang
// embeds it and adds the graph interfaces.
type depLang struct{}

var depKind = rule.KindInfo{
	Name:           "dep_library",
	MatchAny:       true,
	NonEmptyAttrs:  map[string]bool{"srcs": true, "deps": true},
	MergeableAttrs: map[string]bool{"srcs": true},
	ResolveAttrs:   map[string]bool{"deps": true},
}

func (depLang) Name() string                                { return "dep" }
func (depLang) Kinds() []rule.KindInfo                      { return []rule.KindInfo{depKind} }
func (depLang) Fix(context.Context, language.FixArgs) error { return nil }

func (depLang) Generate(_ context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	data, err := os.ReadFile(filepath.Join(args.Dir, "lib.dep"))
	if errors.Is(err, os.ErrNotExist) {
		return language.GenerateResult{}, nil
	} else if err != nil {
		return language.GenerateResult{}, err
	}
	var imports []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			imports = append(imports, line)
		}
	}
	r := rule.NewRule("dep_library", path.Base(args.Dir))
	r.SetAttr("srcs", []string{"lib.dep"})
	return language.GenerateResult{Gen: []*rule.Rule{r}, Imports: []any{imports}}, nil
}

func (depLang) Imports(_ context.Context, args resolve.ImportsArgs) (resolve.ImportsResult, error) {
	return resolve.ImportsResult{Imports: []resolve.ImportSpec{{Lang: "dep", Imp: args.File.Pkg}}}, nil
}

func (depLang) Resolve(ctx context.Context, args resolve.ResolveArgs) error {
	var deps []string
	for _, imp := range args.Imports.([]string) {
		rs, err := args.Index.Find(ctx, args.Config, resolve.ImportSpec{Lang: "dep", Imp: imp}, "dep")
		if err != nil {
			return err
		}
		if len(rs) == 1 && !rs[0].IsSelfImport(args.From) {
			deps = append(deps, rs[0].Label.Rel(args.From.Repo, args.From.Pkg).String())
		}
	}
	if len(deps) > 0 {
		args.Rule.SetAttr("deps", deps)
	} else {
		args.Rule.DelAttr("deps")
	}
	return nil
}

type graphDepLang struct{ depLang }

var (
	_ graph.Contributor  = graphDepLang{}
	_ graph.Materializer = graphDepLang{}
)

func (graphDepLang) ContributeGraph(_ context.Context, args graph.ContributeArgs) error {
	for i, node := range args.Nodes {
		if node == "" {
			continue
		}
		for _, imp := range args.Imports[i].([]string) {
			if err := args.Facts.Requires(node, graph.ImportSpec{Lang: "dep", Imp: imp}, graph.Requires{Attr: "deps"}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (graphDepLang) Materialize(_ context.Context, args graph.MaterializeArgs) error {
	deps := args.Graph.DirectDeps(args.Node, graph.DepQuery{Attrs: []string{"deps"}, Select: graph.Unconditional})
	var strs []string
	for _, l := range graph.RelativeLabels(deps, args.From) {
		strs = append(strs, l.String())
	}
	if len(strs) > 0 {
		args.Rule.SetAttr("deps", strs)
	} else {
		args.Rule.DelAttr("deps")
	}
	return nil
}

// runGazelle writes files into a new directory, runs Gazelle, and returns the
// directory, Run's error, and the log output.
func runGazelle(t *testing.T, langs []language.Language, files map[string]string, args ...string) (string, error, string) {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return dir, runIn(t, dir, langs, args...), lastLog
}

var lastLog string

func runIn(t *testing.T, dir string, langs []language.Language, args ...string) error {
	t.Helper()
	oldVersion := MajorVersion
	MajorVersion = 2
	defer func() { MajorVersion = oldVersion }()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	err := Run(context.Background(), langs, dir, args)
	lastLog = buf.String()
	return err
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
}

func readBuild(t *testing.T, dir, pkg string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(pkg), "BUILD.bazel"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var acyclic = map[string]string{
	"MODULE.bazel": "",
	"a/lib.dep":    "b\nc\nmissing\n",
	"b/lib.dep":    "c\n",
	"c/lib.dep":    "",
	// An existing rule with a hand-written dependency and a kept one.
	"d/BUILD.bazel": `dep_library(
    name = "d",
    srcs = ["lib.dep"],
    deps = [
        "//c",  # keep
    ],
)
`,
	"d/lib.dep": "b\n",
}

// TestGraphParity checks that a graph-aware extension produces the same
// build files as the legacy path.
func TestGraphParity(t *testing.T) {
	legacyDir, err, _ := runGazelle(t, []language.Language{depLang{}}, acyclic, "-index=all")
	if err != nil {
		t.Fatal(err)
	}
	graphDir, err, logs := runGazelle(t, []language.Language{graphDepLang{}}, acyclic, "-index=all", "-experimental_build_graph")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"a", "b", "c", "d"} {
		if diff := cmp.Diff(readBuild(t, legacyDir, pkg), readBuild(t, graphDir, pkg)); diff != "" {
			t.Errorf("%s/BUILD.bazel differs (-legacy +graph):\n%s", pkg, diff)
		}
	}
	if got := readBuild(t, graphDir, "a"); !strings.Contains(got, `deps = [
        "//b",
        "//c",
    ]`) {
		t.Errorf("a/BUILD.bazel:\n%s", got)
	}
	if !strings.Contains(logs, `//a: unresolved import: no target provides "missing"`) {
		t.Errorf("unresolved import not reported; log:\n%s", logs)
	}
	if strings.Contains(logs, "cycle") {
		t.Errorf("unexpected cycle report:\n%s", logs)
	}
}

var cyclic = map[string]string{
	"MODULE.bazel": "",
	"a/lib.dep":    "b\n",
	"b/lib.dep":    "c\n",
	"c/lib.dep":    "a\n",
	"x/lib.dep":    "",
}

// TestGraphCycles checks cycle reporting for graph-aware extensions and,
// through observed edges, for legacy extensions.
func TestGraphCycles(t *testing.T) {
	for _, tc := range []struct {
		name string
		lang language.Language
	}{
		{"graph", graphDepLang{}},
		{"legacy", depLang{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err, logs := runGazelle(t, []language.Language{tc.lang}, cyclic, "-index=all", "-experimental_build_graph")
			if err != nil {
				t.Fatalf("warning mode should not fail: %v", err)
			}
			for _, want := range []string{
				"warningdependency cycle among 3 target(s):",
				`//a --deps [imports "b"]--> //b`,
				`//c --deps [imports "a"]--> //a`,
			} {
				if tc.name == "legacy" {
					// Observed edges have no import reasons.
					want = strings.ReplaceAll(want, ` [imports "b"]`, " [generated]")
					want = strings.ReplaceAll(want, ` [imports "a"]`, " [generated]")
				}
				if !strings.Contains(logs, want) {
					t.Errorf("log missing %q:\n%s", want, logs)
				}
			}

			// Strict error mode fails and doesn't write files.
			dir, err, logs := runGazelle(t, []language.Language{tc.lang}, cyclic, "-index=all", "-experimental_build_graph", "-graph_cycles=error", "-strict")
			if !errors.Is(err, ExitError) {
				t.Errorf("strict error mode: got %v, want ExitError; log:\n%s", err, logs)
			}
			if _, err := os.Stat(filepath.Join(dir, "a", "BUILD.bazel")); err == nil {
				t.Error("build files were written despite a cycle error in strict mode")
			}

			// The directive overrides the flag; "off" reports nothing.
			files := map[string]string{"BUILD.bazel": "# gazelle:graph_cycles off\n"}
			for k, v := range cyclic {
				files[k] = v
			}
			_, _, logs = runGazelle(t, []language.Language{tc.lang}, files, "-index=all", "-experimental_build_graph")
			if strings.Contains(logs, "cycle") {
				t.Errorf("graph_cycles off still reported:\n%s", logs)
			}

			// Without the flag, nothing changes.
			_, _, logs = runGazelle(t, []language.Language{tc.lang}, cyclic, "-index=all")
			if strings.Contains(logs, "cycle") {
				t.Errorf("cycle reported without -experimental_build_graph:\n%s", logs)
			}
		})
	}
}

// TestGraphIdempotent runs Gazelle twice: the second run starts from the
// build files the first one wrote. The build files and the graph (excluding
// origins) must not change.
func TestGraphIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name string
		lang language.Language
	}{
		{"graph", graphDepLang{}},
		{"legacy", depLang{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, acyclic)
			jsonPath := func(i int) string { return filepath.Join(t.TempDir(), "graph.json") }
			var graphs []string
			var builds []string
			for i := range 2 {
				p := jsonPath(i)
				if err := runIn(t, dir, []language.Language{tc.lang}, "-index=all", "-experimental_build_graph", "-graph_json="+p); err != nil {
					t.Fatal(err)
				}
				graphs = append(graphs, structure(t, p))
				var all string
				for _, pkg := range []string{"a", "b", "c", "d"} {
					all += readBuild(t, dir, pkg)
				}
				builds = append(builds, all)
			}
			if diff := cmp.Diff(builds[0], builds[1]); diff != "" {
				t.Errorf("build files changed on second run:\n%s", diff)
			}
			if diff := cmp.Diff(graphs[0], graphs[1]); diff != "" {
				t.Errorf("graph changed on second run:\n%s", diff)
			}
		})
	}
}

// structure reads a graph JSON dump and strips origins, which legitimately
// differ between a run that creates build files and one that reads them.
func structure(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string][]map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	for _, list := range v {
		for _, item := range list {
			delete(item, "origin")
		}
	}
	out, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestGraphIndexedDirectory checks construction from existing build files:
// when only "a" is updated, its dependency on "b" is linked through b's
// existing BUILD file, and a cycle through b's hand-written deps is found.
func TestGraphIndexedDirectory(t *testing.T) {
	files := map[string]string{
		"MODULE.bazel": "",
		"a/lib.dep":    "b\n",
		"b/BUILD.bazel": `dep_library(
    name = "b",
    srcs = ["lib.dep"],
    deps = ["//a"],
)
`,
	}
	dir, err, logs := runGazelle(t, []language.Language{graphDepLang{}}, files, "-index=all", "-experimental_build_graph", "a")
	if err != nil {
		t.Fatal(err)
	}
	if got := readBuild(t, dir, "a"); !strings.Contains(got, `deps = ["//b"]`) {
		t.Errorf("a/BUILD.bazel:\n%s", got)
	}
	if !strings.Contains(logs, "//b --deps [indexed]--> //a") {
		t.Errorf("cycle through indexed build file not reported:\n%s", logs)
	}
}

// A generated rule merged into an existing wrapper macro (alias_kind) must
// not keep the wrapper's stale deps as facts.
func TestGraphAliasKindStaleDeps(t *testing.T) {
	files := map[string]string{
		"MODULE.bazel": "",
		"BUILD.bazel":  "# gazelle:alias_kind my_dep_library dep_library\n",
		"a/lib.dep":    "",
		"a/BUILD.bazel": `my_dep_library(
    name = "a",
    srcs = ["lib.dep"],
    deps = ["//b"],
)
`,
		"b/lib.dep": "a\n",
	}
	_, err, logs := runGazelle(t, []language.Language{graphDepLang{}}, files, "-index=all", "-experimental_build_graph")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs, "cycle") {
		t.Errorf("stale dep from wrapper macro produced a cycle:\n%s", logs)
	}
}

func TestGraphDirective(t *testing.T) {
	files := map[string]string{"MODULE.bazel": "", "a/BUILD.bazel": "# gazelle:graph_cycles error\n"}
	_, err, logs := runGazelle(t, []language.Language{graphDepLang{}}, files, "-experimental_build_graph")
	if err == nil && !strings.Contains(logs, "only be set in the root") {
		t.Errorf("subdirectory directive accepted; log:\n%s", logs)
	}

	// Gazelle v1 doesn't know the directive.
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"MODULE.bazel": "", "BUILD.bazel": "# gazelle:graph_cycles bogus\n"})
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	old := MajorVersion
	MajorVersion = 1
	defer func() { MajorVersion = old }()
	Run(context.Background(), []language.Language{depLang{}}, dir, nil)
	if !strings.Contains(buf.String(), "unknown directive") {
		t.Errorf("v1 should report graph_cycles as unknown; log:\n%s", buf.String())
	}
}

func TestGraphDumps(t *testing.T) {
	out := t.TempDir()
	jsonPath, dotPath := filepath.Join(out, "g.json"), filepath.Join(out, "g.dot")
	_, err, _ := runGazelle(t, []language.Language{graphDepLang{}}, acyclic, "-index=all", "-experimental_build_graph", "-graph_json="+jsonPath, "-graph_dot="+dotPath)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{jsonPath: `"t:@@//a:a"`, dotPath: `"t:@@//a:a" -> "t:@@//b:b" [label="deps"]`} {
		data, err := os.ReadFile(p)
		if err != nil || !strings.Contains(string(data), want) {
			t.Errorf("%s missing %q (err %v)", p, want, err)
		}
	}
}

// An explicit -graph_cycles flag overrides the root directive.
func TestGraphCyclesFlagOverridesDirective(t *testing.T) {
	files := map[string]string{"BUILD.bazel": "# gazelle:graph_cycles off\n"}
	for k, v := range cyclic {
		files[k] = v
	}
	_, _, logs := runGazelle(t, []language.Language{graphDepLang{}}, files, "-index=all", "-experimental_build_graph", "-graph_cycles=warning")
	if !strings.Contains(logs, "dependency cycle") {
		t.Errorf("explicit -graph_cycles=warning was overridden by the directive:\n%s", logs)
	}
}
