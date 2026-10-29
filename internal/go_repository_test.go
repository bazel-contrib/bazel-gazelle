/* Copyright 2019 The Bazel Authors. All rights reserved.

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

package bazel_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

// The BUILD file sets ruleFiles at link time. Passing all of //:all_files on
// the test command line exceeds the Windows process limit.
var ruleFiles string

var testArgs = bazel_testing.Args{
	ModuleFileSuffix: `
bazel_dep(name = "rules_go", version = "0.59.0", repo_name = "io_bazel_rules_go")
repos = use_extension("//:repos.bzl", "repos")
use_repo(repos, "consumer")
`,
	Main: `
-- BUILD.bazel --
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix example.com/m

exports_files(["config.json"])

gazelle(name = "gazelle")

gazelle(
    name = "gazelle-update-repos",
    args = [
        "-from_file=go.mod",
        "-to_macro=deps.bzl%go_repositories",
    ],
    command = "update-repos",
)

-- go.mod --
module example.com/m

go 1.15
-- hello.go --
package main

func main() {}

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
	WorkspacePrefix: `
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "68af54cb97fbdee5e5e8fe8d210d15a518f9d62abfd71620c3eaff3b26a5ff86",
    urls = [
        "https://mirror.bazel.build/github.com/bazel-contrib/rules_go/releases/download/v0.59.0/rules_go-v0.59.0.zip",
        "https://github.com/bazel-contrib/rules_go/releases/download/v0.59.0/rules_go-v0.59.0.zip",
    ],
)
`,
	WorkspaceSuffix: `
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies(
	go_env = {
		"GOPRIVATE": "example.com/m",
		"GOSUMDB": "off",
	},
	go_env_inherit = ["GAZELLE_INHERITED_TOKEN"],
)

# gazelle:repo test

go_repository(
    name = "errors_go_git",
    importpath = "github.com/pkg/errors",
    commit = "30136e27e2ac8d167177e8a583aa4c3fea5be833",
    patches = ["@bazel_gazelle//internal:repository_rules_test_errors.patch"],
    patch_args = ["-p1"],
    build_naming_convention = "go_default_library",
)

go_repository(
    name = "errors_go_mod",
    importpath = "github.com/pkg/errors",
    version = "v0.8.1",
    sum ="h1:iURUrRGxPUNPdy5/HRSm+Yj6okJ6UtLINN0Q9M4+h3I=",
)

go_repository(
    name = "org_golang_x_xerrors",    
		importpath = "golang.org/x/xerrors",
    sum = "h1:go1bK/D/BFZV2I8cIQd1NKEZ+0owSTG1fDTci4IqFcE=",
    version = "v0.0.0-20200804184101-5ec99f83aff1",
)

go_repository(
    name = "com_github_apex_log",
    build_directives = [
        "gazelle:exclude handlers",
        "gazelle:default_visibility @com_github_apex_log//:__subpackages__",
    ],
    importpath = "github.com/apex/log",
    sum = "h1:J5rld6WVFi6NxA6m8GJ1LJqu3+GiTFIt3mYv27gdQWI=",
    version = "v1.1.0",
)
`,
}

func TestMain(m *testing.M) {
	args := append([]string{os.Args[0], "-begin_files"}, strings.Fields(ruleFiles)...)
	args = append(args, "-end_files")
	os.Args = append(args, os.Args[1:]...)

	if err := os.Setenv("GAZELLE_INHERITED_TOKEN", "top-secret-token"); err != nil {
		panic(err)
	}
	// ModuleFileSuffix makes bazel_testing default to Bzlmod. Retain the
	// WORKSPACE default for the existing tests; TestUnrelatedRepositoryDoesNotRefetch
	// opts into Bzlmod on its query commands.
	flags := "--noenable_bzlmod " + os.Getenv("GO_BAZEL_TEST_BAZELFLAGS")
	if err := os.Setenv("GO_BAZEL_TEST_BAZELFLAGS", flags); err != nil {
		panic(err)
	}
	bazel_testing.TestMain(m, testArgs)
}

func TestBuild(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "--enable_workspace", "@errors_go_git//:errors", "@errors_go_mod//:go_default_library"); err != nil {
		t.Fatal(err)
	}
}

func TestExcludeDirective(t *testing.T) {
	err := bazel_testing.RunBazel("query", "--enable_workspace", "@com_github_apex_log//handlers/...")
	if err == nil {
		t.Fatal("Should not generate build files for @com_github_apex_log//handlers/...")
	}
	if !strings.Contains(err.Error(), "no targets found beneath 'handlers'") {
		t.Fatal("Unexpected error:\n", err)
	}
}

func TestDefaultVisibilityDirective(t *testing.T) {
	output, err := bazel_testing.BazelOutput("query", "--output=streamed_jsonproto", "--enable_workspace", "@com_github_apex_log//:log")
	if err != nil {
		t.Fatalf("bazel query failed: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("got %d lines of query output; want 1", len(lines))
	}
	var target struct {
		Rule struct {
			Attribute []struct {
				Name            string   `json:"name"`
				StringListValue []string `json:"stringListValue"`
			} `json:"attribute"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(lines[0], &target); err != nil {
		t.Fatalf("decoding streamed_jsonproto: %v", err)
	}
	var visibilities []string
	for _, attr := range target.Rule.Attribute {
		if attr.Name == "visibility" {
			visibilities = append(visibilities, attr.StringListValue...)
		}
	}
	got := strings.Join(visibilities, ",")
	want := "@com_github_apex_log//:__subpackages__"
	if got != want {
		t.Errorf("got visibility %s; want %s", got, want)
	}
}

func TestRepoConfig(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "--enable_workspace", "@bazel_gazelle_go_repository_config//:all"); err != nil {
		t.Fatal(err)
	}
	outputBase, err := getBazelOutputBase()
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(outputBase, "external/bazel_gazelle_go_repository_config")
	testtools.CheckFiles(t, outDir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
# Code generated by generate_repo_config.go; DO NOT EDIT.

go_repository(
    name = "com_github_apex_log",
    importpath = "github.com/apex/log",
)

go_repository(
    name = "errors_go_git",
    build_naming_convention = "go_default_library",
    importpath = "github.com/pkg/errors",
)

go_repository(
    name = "errors_go_mod",
    importpath = "github.com/pkg/errors",
)

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://mirror.bazel.build/github.com/bazel-contrib/rules_go/releases/download/v0.59.0/rules_go-v0.59.0.zip",
        "https://github.com/bazel-contrib/rules_go/releases/download/v0.59.0/rules_go-v0.59.0.zip",
    ],
)

go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
)
`,
		},
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
	archivePath := filepath.Join(t.TempDir(), "consumer.zip")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove("config.json"); err != nil && !os.IsNotExist(err) {
			t.Errorf("removing config.json: %v", err)
		}
	})
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
		{"input_component", "example.com/dependency/input", "input", "renamed//pkg", true},
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

func TestGoModCacheModes(t *testing.T) {
	const (
		repo       = "errors_go_mod"
		importpath = "github.com/pkg/errors"
		version    = "v0.8.1"
		target     = "@errors_go_mod//:go_default_library"
	)
	outputBase, err := getBazelOutputBase()
	if err != nil {
		t.Fatal(err)
	}
	sharedModcache := filepath.Join(outputBase, "external/bazel_gazelle_go_repository_cache/pkg/mod")

	cases := []struct {
		name        string
		extraArgs   []string
		wantTree    bool
		wantArchive bool
	}{
		{
			name: "default",
			// Explicit "0" (vs unset) invalidates Bazel's repo cache so the
			// pre-clean's removal of shared-cache state gets repopulated.
			extraArgs:   []string{"--repo_env=GO_REPOSITORY_EPHEMERAL_MODCACHE=0"},
			wantTree:    true,
			wantArchive: true,
		},
		{
			name:        "ephemeral",
			extraArgs:   []string{"--repo_env=GO_REPOSITORY_EPHEMERAL_MODCACHE=1"},
			wantTree:    false,
			wantArchive: false,
		},
		{
			name: "host_modcache",
			extraArgs: []string{
				"--repo_env=GO_REPOSITORY_USE_HOST_MODCACHE=1",
				"--repo_env=GOMODCACHE=" + t.TempDir(),
			},
			wantTree:    true,
			wantArchive: true,
		},
		{
			name: "host_cache",
			extraArgs: []string{
				"--repo_env=GO_REPOSITORY_USE_HOST_CACHE=1",
				"--repo_env=GOMODCACHE=" + t.TempDir(),
				"--repo_env=GOCACHE=" + t.TempDir(),
			},
			wantTree:    true,
			wantArchive: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear any prior write to the shared cache from another subtest so
			// each case observes only its own fetch.
			for _, p := range []string{
				filepath.Join(sharedModcache, importpath+"@"+version),
				filepath.Join(sharedModcache, "cache/download", importpath),
			} {
				if err := os.RemoveAll(p); err != nil {
					t.Fatalf("pre-clean %s: %v", p, err)
				}
			}

			// If the case overrides GOMODCACHE via --repo_env, that's where the
			// tree/archive should end up; otherwise the shared cache.
			modcache := sharedModcache
			for _, arg := range tc.extraArgs {
				if v, ok := strings.CutPrefix(arg, "--repo_env=GOMODCACHE="); ok {
					modcache = v
					break
				}
			}

			args := append([]string{"query", "--enable_workspace"}, tc.extraArgs...)
			args = append(args, target)

			if err := bazel_testing.RunBazel(args...); err != nil {
				t.Fatal(err)
			}

			treePath := filepath.Join(modcache, importpath+"@"+version)
			_, treeErr := os.Stat(treePath)
			switch {
			case tc.wantTree && treeErr != nil:
				t.Errorf("expected extracted tree at %s: %v", treePath, treeErr)
			case !tc.wantTree && treeErr == nil:
				t.Errorf("extracted tree at %s should not exist", treePath)
			}

			zipPath := filepath.Join(modcache, "cache/download", importpath, "@v", version+".zip")
			zipInfo, zipErr := os.Stat(zipPath)
			switch {
			case tc.wantArchive && zipErr != nil:
				t.Errorf("expected module zip at %s: %v", zipPath, zipErr)
			case tc.wantArchive && zipInfo.Mode()&0o200 == 0:
				t.Errorf("module zip at %s is not writable (mode %v); -modcacherw missing?", zipPath, zipInfo.Mode())
			case !tc.wantArchive && zipErr == nil:
				t.Errorf("module zip at %s should not exist", zipPath)
			}

			// A leaked GOMODCACHE tempdir under the repo looks like a `cache/download` subtree.
			repoRoot := filepath.Join(outputBase, "external", repo)
			walkErr := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && filepath.Base(path) == "download" && filepath.Base(filepath.Dir(path)) == "cache" {
					t.Errorf("leaked GOMODCACHE tree: %s", path)
					return filepath.SkipDir
				}
				return nil
			})
			if walkErr != nil {
				t.Fatalf("walking %s: %v", repoRoot, walkErr)
			}
		})
	}
}

func TestRepoCacheContainsGoEnv(t *testing.T) {
	if err := bazel_testing.RunBazel("query", "--enable_workspace", "@errors_go_mod//:go_default_library"); err != nil {
		t.Fatal(err)
	}
	outputBase, err := getBazelOutputBase()
	if err != nil {
		t.Fatal(err)
	}
	goEnvPath := filepath.Join(outputBase, "external/bazel_gazelle_go_repository_cache", "go.env")
	gotBytes, err := os.ReadFile(goEnvPath)
	if err != nil {
		t.Fatalf("could not read file %s: %v", goEnvPath, err)
	}
	for _, want := range []string{
		"GOPRIVATE='example.com/m'",
		"GOSUMDB='off'",
		"GAZELLE_INHERITED_TOKEN='top-secret-token'",
	} {
		if !strings.Contains(string(gotBytes), want) {
			t.Fatalf("go.env did not contain %s", want)
		}
	}

	// The cache env must stay relocatable so that vendoring across users can use said go.env.
	for _, line := range strings.Split(string(gotBytes), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			t.Fatalf("malformed go.env line %q", line)
		}
		value = strings.Trim(value, "'")
		if filepath.IsAbs(value) {
			t.Errorf("go.env: %s=%s is an absolute path; go.env must be relocatable", key, value)
		}
		if strings.Contains(value, outputBase) {
			t.Errorf("go.env: %s=%s references the output base; go.env must be relocatable", key, value)
		}
	}

	goEnvBzlPath := filepath.Join(outputBase, "external/bazel_gazelle_go_repository_config", "go_env.bzl")
	goEnvBzl, err := os.ReadFile(goEnvBzlPath)
	if err != nil {
		t.Fatalf("could not read file %s: %v", goEnvBzlPath, err)
	}
	if !strings.Contains(string(goEnvBzl), "\"GAZELLE_INHERITED_TOKEN\": \"top-secret-token\"") {
		t.Fatalf("go_env.bzl did not contain inherited GAZELLE_INHERITED_TOKEN")
	}
}

// TODO(bazelbuild/rules_go#2189): call bazel_testing.BazelOutput once implemented.
func getBazelOutputBase() (string, error) {
	cmd := exec.Command("bazel", "info", "output_base")
	for _, e := range os.Environ() {
		// Filter environment variables set by the bazel test wrapper script.
		// These confuse recursive invocations of Bazel.
		if strings.HasPrefix(e, "TEST_") || strings.HasPrefix(e, "RUNFILES_") {
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
