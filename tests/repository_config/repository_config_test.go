// Copyright 2026 The Bazel Authors. All rights reserved.
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

package repository_config_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

// The BUILD file sets ruleFiles at link time. Passing all of //:all_files on
// the TestRunner command line exceeds the Windows process limit.
var ruleFiles string

func TestMain(m *testing.M) {
	args := append([]string{os.Args[0], "-begin_files"}, strings.Fields(ruleFiles)...)
	args = append(args, "-end_files")
	os.Args = append(args, os.Args[1:]...)
	bazel_testing.TestMain(m, bazel_testing.Args{
		ModuleFileSuffix: `
bazel_dep(name = "rules_go", version = "0.59.0", repo_name = "io_bazel_rules_go")
repos = use_extension("//:repos.bzl", "repos")
use_repo(repos, "consumer")
`,
		Main: `
-- BUILD.bazel --
exports_files(["config.json"])
-- repos.bzl --
load("@bazel_gazelle//internal:go_repository.bzl", "go_repository")
load("@bazel_gazelle//internal/bzlmod:go_repository_config.bzl", "go_repository_config")

def _stub_impl(ctx):
    ctx.file("BUILD.bazel", "")

_stub = repository_rule(implementation = _stub_impl)

def _repos(ctx):
    config = json.decode(ctx.read(Label("//:config.json")))
    for name in config["repos"].values():
        if name != "consumer":
            _stub(name = name)
    go_repository_config(
        name = "bazel_gazelle_go_repository_config",
        repo_names = config["repos"],
        module_names = {},
        prefix_dirs = {},
        build_naming_conventions = {},
        go_env = {},
        tool_targets = {},
    )
    go_repository(
        name = "consumer",
        importpath = "example.com/consumer",
        internal_only_do_not_use_apparent_name = "consumer",
        urls = [config["url"]],
        sha256 = config["sha256"],
        build_external = "static",
    )

repos = module_extension(implementation = _repos)
`,
	})
}

func TestUnrelatedRepositoryDoesNotRefetch(t *testing.T) {
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	f, err := w.Create("lib.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("package consumer\nimport (_ \"example.com/dependency/pkg\"; _ \"unknown.dev/new\")\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath, err := filepath.Abs("consumer.zip")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	uriPath := filepath.ToSlash(archivePath)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	archiveURL := (&url.URL{Scheme: "file", Path: uriPath}).String()
	repos := map[string]string{
		"example.com/consumer": "consumer",
	}
	for _, tc := range []struct {
		name, module, repo, label string
		wantExtraction            bool
	}{
		{"populate", "example.com/dependency", "dependency", "dependency//pkg", true},
		{"unrelated", "example.com/other", "unrelated", "dependency//pkg", false},
		{"child_without_internal", "example.com/consumer/child", "child", "dependency//pkg", false},
		{"rename", "example.com/dependency", "renamed", "renamed//pkg", true},
		{"nested", "example.com/dependency/pkg", "nested", "nested//:pkg", true},
		{"new_host", "unknown.dev/new", "added", "added//:new", true},
		{"remove_nested", "example.com/dependency/pkg", "", "renamed//pkg", true},
	} {
		if !t.Run(tc.name, func(t *testing.T) {
			if tc.repo != "" {
				repos[tc.module] = tc.repo
			} else {
				delete(repos, tc.module)
			}
			config, err := json.Marshal(map[string]any{
				"repos": repos, "url": archiveURL, "sha256": fmt.Sprintf("%x", sha256.Sum256(archive.Bytes())),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("config.json", config, 0o666); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(t.TempDir(), "profile.json")
			out, err := bazel_testing.BazelOutput("query", "--enable_bzlmod", "--profile="+profile, "--output=build", "@consumer//:consumer")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			var trace struct {
				Events []struct{ Name string } `json:"traceEvents"`
			}
			if err := json.Unmarshal(data, &trace); err != nil {
				t.Fatal(err)
			}
			extracted := false
			for _, event := range trace.Events {
				if strings.HasPrefix(event.Name, "extracting: repository ") && strings.HasSuffix(event.Name, "consumer") {
					extracted = true
				}
			}
			if extracted != tc.wantExtraction {
				t.Errorf("repository extraction = %v, want %v", extracted, tc.wantExtraction)
			}
			if !strings.Contains(string(out), tc.label) {
				t.Errorf("generated dependency %s not found:\n%s", tc.label, out)
			}
		}) {
			return
		}
	}
}
