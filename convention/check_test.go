// Copyright 2017 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package convention

import (
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	golang "github.com/bazelbuild/bazel-gazelle/language/go"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

type testConvention struct {
}

func (*testConvention) CheckConvention(c *config.Config, kind, imp, name, rel string) bool {
	return kind == "go_library"
}

func TestAddRule(t *testing.T) {
	goResolve := golang.NewLanguage()
	protoResolve := proto.NewLanguage()
	msrlv := func(r *rule.Rule, pkgRel string) resolve.Resolver {
		switch r.Kind() {
		case "proto_library":
			return protoResolve
		case "go_library":
			return goResolve
		default:
			return nil
		}
	}

	c := config.New()
	f := &rule.File{Pkg: ""}
	c.Exts[_conventionName] = &conventionConfig{genResolves: true}
	chk := NewChecker(c, nil, msrlv, &testConvention{})

	var wantResolves []resolveSpec

	// conventional go_library rule
	r := rule.NewRule("go_library", "")
	r.SetAttr("importpath", "code.internal/foo")
	chk.AddRule(c, r, f)

	// unconventional proto_library rule
	r = rule.NewRule("proto_library", "")
	r.SetAttr("srcs", rule.ExprFromValue([]string{"foo/bar.proto"}))
	wantResolves = append(wantResolves, resolveSpec{
		imps: []resolve.ImportSpec{
			resolve.ImportSpec{Lang: "proto", Imp: "foo/bar.proto"},
		},
		label: label.New("", f.Pkg, r.Name()),
		file:  f,
		rule:  r,
	})
	chk.AddRule(c, r, f)

	// rule with no Resolver is ignored
	r = rule.NewRule("foo_library", "")
	chk.AddRule(c, r, f)

	if !reflect.DeepEqual(wantResolves, chk.resolves) {
		t.Errorf("got resolves %v; want %v", chk.resolves, wantResolves)
	}
}

func TestParseDirective(t *testing.T) {
	tests := []struct {
		name          string
		inLine        string
		wantDirective directive
		wantError     bool
	}{
		{
			name:   "proper format",
			inLine: "# gazelle:resolve go go example.com/foo //src/foo:go_default_library",
			wantDirective: directive{
				imp:   resolve.ImportSpec{Imp: "example.com/foo", Lang: "go"},
				label: label.New("", "src/foo", "go_default_library"),
			},
		},
		{
			name:      "missing lang",
			inLine:    "# gazelle:resolve example.com/foo //src/foo:go_default_library",
			wantError: true,
		},
		{
			name:      "extra space",
			inLine:    " # gazelle:resolve go example.com/foo //src/foo:go_default_library",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDirective, err := parseDirective(tt.inLine)
			if tt.wantError {
				if err == nil {
					t.Error("err should not be nil")
				}
				return
			}
			if err != nil {
				t.Errorf("err %v should not be nil", err)
			}
			if tt.wantDirective != gotDirective {
				t.Errorf("got directive %v; want %v", gotDirective, tt.wantDirective)
			}
		})
	}
}

func TestFinish(t *testing.T) {
	mrslv := func(r *rule.Rule, pkgRel string) resolve.Resolver {
		return nil
	}

	startContent := `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/baz //src/code.internal/bar:go_default_library
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
`
	tests := []struct {
		name           string
		inContent      string
		inResolveSpecs []resolveSpec
		wantContent    string
		dirs           []string
		recursive      bool
	}{
		{
			name:        "no new resolves",
			inContent:   startContent,
			wantContent: startContent,
			dirs:        []string{"src/code.internal/devexp"},
		},
		{
			name:      "new resolves",
			inContent: startContent,
			inResolveSpecs: []resolveSpec{
				// unconventional go_library rule
				resolveSpec{
					imps:  []resolve.ImportSpec{resolve.ImportSpec{Imp: "code.internal/foo/baz", Lang: "go"}},
					label: label.New("", "src/code.internal/foo/bar", "go_default_library"),
					rule:  &rule.Rule{},
					file:  &rule.File{},
				},
				// unconventional proto rule
				resolveSpec{
					imps: []resolve.ImportSpec{
						resolve.ImportSpec{Imp: "idl/foo/bar.proto", Lang: "proto"},
						resolve.ImportSpec{Imp: "idl/foo/baz.proto", Lang: "proto"},
					},
					label: label.New("", "idl/foo", "foopb_proto"),
					rule:  &rule.Rule{},
					file:  &rule.File{},
				},
			},
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/baz //src/code.internal/bar:go_default_library
# gazelle:resolve go go code.internal/foo/baz //src/code.internal/foo/bar:go_default_library
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
# gazelle:resolve proto proto idl/foo/baz.proto //idl/foo:foopb_proto
`,
		},
		{
			name:      "prune on full repo run",
			inContent: startContent,
			inResolveSpecs: []resolveSpec{
				// unconventional go_library rule
				resolveSpec{
					imps:  []resolve.ImportSpec{resolve.ImportSpec{Imp: "code.internal/foo/baz", Lang: "go"}},
					label: label.New("", "src/code.internal/foo/bar", "go_default_library"),
					rule:  &rule.Rule{},
					file:  &rule.File{},
				},
				// unconventional proto rule
				resolveSpec{
					imps: []resolve.ImportSpec{
						resolve.ImportSpec{Imp: "idl/foo/bar.proto", Lang: "proto"},
						resolve.ImportSpec{Imp: "idl/foo/baz.proto", Lang: "proto"},
					},
					label: label.New("", "idl/foo", "foopb_proto"),
					rule:  &rule.Rule{},
					file:  &rule.File{},
				},
			},
			dirs:      []string{""},
			recursive: true,
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/foo/baz //src/code.internal/foo/bar:go_default_library
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
# gazelle:resolve proto proto idl/foo/baz.proto //idl/foo:foopb_proto
`,
		},
		{
			name:      "prune for subtree",
			inContent: startContent,
			dirs:      []string{"src"},
			recursive: true,
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
`,
		},
		{
			name: "prune for one directory but not its subdirectories",
			inContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/foo/baz //src/code.internal/foo/bar:go_default_library
# gazelle:resolve go go code.internal/foo //src/code.internal/foo:go_mock_library
`,
			dirs: []string{"src/code.internal/foo"},
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/foo/baz //src/code.internal/foo/bar:go_default_library
`,
		},
		{
			name:      "should not prune outside given dirs",
			inContent: startContent,
			dirs:      []string{"src/code.internal/bar"},
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
`,
		},
		{
			name:      "update import path",
			inContent: startContent,
			inResolveSpecs: []resolveSpec{
				{
					imps: []resolve.ImportSpec{
						{Imp: "code.internal/foo/baz", Lang: "go"},
					},
					label: label.New("", "src/code.internal/bar", "go_default_library"),
					rule:  &rule.Rule{},
					file:  &rule.File{},
				},
			},
			dirs: []string{"src/code.internal/bar"},
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/foo/baz //src/code.internal/bar:go_default_library
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
`,
		},
		{
			name:      "update label",
			inContent: startContent,
			inResolveSpecs: []resolveSpec{
				{
					imps: []resolve.ImportSpec{
						{Imp: "code.internal/baz", Lang: "go"},
					},
					label: label.New("", "src/code.internal/bar/foo", "go_default_library"),
					rule:  &rule.Rule{},
					file:  &rule.File{},
				},
			},
			dirs: []string{"src/code.internal/bar"},
			wantContent: `# do not edit
### AUTOMATIC RESOLVES ###
# gazelle:resolve go go code.internal/baz //src/code.internal/bar/foo:go_default_library
# gazelle:resolve proto proto idl/foo/bar.proto //idl/foo:foopb_proto
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.New()
			c.Exts[_conventionName] = &conventionConfig{
				genResolves:   true,
				recursiveMode: tt.recursive,
			}
			cext := resolve.Configurer{}
			cext.RegisterFlags(nil, "", c)

			tmpDir := t.TempDir()
			c.RepoRoot = tmpDir
			buildFile := path.Join(tmpDir, "BUILD.bazel")
			if err := os.WriteFile(buildFile, []byte(tt.inContent), 0644); err != nil {
				t.Fatalf("failed to write BUILD.bazel: %v", err)
			}
			chk := NewChecker(c, tt.dirs, mrslv, &testConvention{})
			chk.resolves = tt.inResolveSpecs
			chk.Finish(c, resolve.NewRuleIndex(mrslv))
			gotContent, err := os.ReadFile(buildFile)
			if err != nil {
				t.Errorf("failed to read file: %v", err)
			}
			if tt.wantContent != string(gotContent) {
				t.Errorf("got content %s; want %s", gotContent, tt.wantContent)
			}
		})
	}
}

func TestReplaceDirectivesInScope(t *testing.T) {
	directiveMap := map[string]directive{
		"uber.com/foo": {
			imp:   resolve.ImportSpec{Lang: "go"},
			label: label.Label{Pkg: "uber.com/foo", Name: "go_default_library"},
		},
	}

	if _, hasOuted := replaceDirectivesInScope(directiveMap, directiveMap, func(s string) bool {
		return true
	}); hasOuted {
		t.Errorf("hasOuted should be false")
	}
}
