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
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

func TestPluginDirectiveGoRepositoryMode(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mode             string
		explicit         bool
		malformed        bool
		wantDirective    bool
		wantDirectiveErr bool
	}{
		{name: "flag_absent", wantDirective: true},
		{name: "flag_false", mode: "false", explicit: true, wantDirective: true},
		{name: "fetch_ignores_directive", mode: "true"},
		{name: "fetch_preserves_explicit_flag", mode: "true", explicit: true},
		{name: "fetch_ignores_malformed_directive", mode: "true", malformed: true},
		{name: "normal_rejects_malformed_directive", malformed: true, wantDirectiveErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := config.New()
			c.RepoRoot = t.TempDir()
			c.WorkDir = c.RepoRoot
			value := "dependency-plugin --option"
			if tc.malformed {
				value = ""
			}
			if err := os.WriteFile(filepath.Join(c.RepoRoot, "BUILD.bazel"), []byte("# gazelle:plugin "+value+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			ld := &Loader{}
			fs := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			ld.RegisterFlags(fs, "fix", c)
			var args []string
			if tc.mode != "" {
				fs.Bool("go_repository_mode", false, "dependency fetch mode")
				args = append(args, "-go_repository_mode="+tc.mode)
			}
			explicitPath := filepath.Join(c.RepoRoot, "workspace-plugin")
			if tc.explicit {
				args = append(args, "-plugin="+explicitPath)
			}
			if err := fs.Parse(args); err != nil {
				t.Fatal(err)
			}
			if err := ld.CheckFlags(fs, c); err != nil {
				t.Fatal(err)
			}
			// Inspect registration only: these paths are never executed.
			specs, err := ld.specs(c)
			if tc.wantDirectiveErr {
				if err == nil || !strings.Contains(err.Error(), "expected a path") {
					t.Fatalf("specs error = %v; want malformed directive error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantCount := 0
			if tc.explicit {
				wantCount++
			}
			if tc.wantDirective {
				wantCount++
			}
			if len(specs) != wantCount {
				t.Fatalf("got %d specs; want %d", len(specs), wantCount)
			}
			if tc.explicit && specs[0].path != explicitPath {
				t.Errorf("explicit plugin = %q; want %q", specs[0].path, explicitPath)
			}
			if tc.wantDirective {
				s := specs[len(specs)-1]
				if s.path != filepath.Join(c.RepoRoot, "dependency-plugin") || len(s.args) != 1 || s.args[0] != "--option" {
					t.Errorf("directive spec = %+v", s)
				}
			}
			f := rule.EmptyFile(filepath.Join(c.RepoRoot, "nested", "BUILD.bazel"), "nested")
			f.Directives = []rule.Directive{{Key: pluginDirective, Value: value}}
			err = ld.Configure(context.Background(), config.ConfigureArgs{Config: c, Rel: "nested", File: f})
			if (err == nil) != (tc.mode == "true") {
				t.Errorf("nested Configure error = %v; fetch mode = %q", err, tc.mode)
			}
		})
	}
}

func TestFindFlagPlugin(t *testing.T) {
	workDir := t.TempDir()
	binDir := filepath.Join(workDir, "tools")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exeName := "my_plugin"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exe := filepath.Join(binDir, exeName)
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := config.New()
	c.WorkDir = workDir
	t.Setenv("PATH", binDir)

	for _, tc := range []struct {
		desc, value, want, wantErr string
	}{
		{desc: "absolute", value: exe, want: exe},
		{desc: "relative_to_workdir", value: "tools/" + exeName, want: exe},
		{desc: "path_lookup", value: "my_plugin", want: exe},
		{desc: "missing", value: "tools/missing", wantErr: "plugin executable not found"},
		{desc: "empty", value: "", wantErr: "expected a path"},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := findFlagPlugin(c, tc.value)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %q, %v; want error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}
