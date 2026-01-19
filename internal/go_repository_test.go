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
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/bazelbuild/rules_go/go/tools/bazel_testing"
)

var testArgs = bazel_testing.Args{
	Main: `
-- BUILD.bazel --
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix example.com/m

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
`,
	ModuleFileSuffix: `
go_deps = use_extension("@bazel_gazelle//:extensions.bzl", "go_deps")
go_deps.config(
	go_env = {
		"GOPRIVATE": "example.com/m",
		"GOSUMDB": "off",
	},
)
go_deps.module(
    path = "github.com/pkg/errors",
    version = "v0.8.1",
    sum = "h1:iURUrRGxPUNPdy5/HRSm+Yj6okJ6UtLINN0Q9M4+h3I=",
)
go_deps.module(
    path = "golang.org/x/xerrors",
    sum = "h1:go1bK/D/BFZV2I8cIQd1NKEZ+0owSTG1fDTci4IqFcE=",
    version = "v0.0.0-20200804184101-5ec99f83aff1",
)
go_deps.module(
    path = "github.com/apex/log",
    sum = "h1:J5rld6WVFi6NxA6m8GJ1LJqu3+GiTFIt3mYv27gdQWI=",
    version = "v1.1.0",
)
go_deps.gazelle_override(
    path = "github.com/apex/log",
    directives = [
        "gazelle:exclude handlers",
        "gazelle:default_visibility //:__subpackages__",
    ],
)
use_repo(
    go_deps,
    "com_github_apex_log",
    "com_github_pkg_errors",
    "org_golang_x_xerrors",
    "bazel_gazelle_go_repository_config",
)
`,
}

func TestMain(m *testing.M) {
	bazel_testing.TestMain(m, testArgs)
}

func TestBuild(t *testing.T) {
	if err := bazel_testing.RunBazel("build", "@com_github_pkg_errors//:go_default_library"); err != nil {
		t.Fatal(err)
	}
}

func TestExcludeDirective(t *testing.T) {
	err := bazel_testing.RunBazel("query", "@com_github_apex_log//handlers/...")
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
	if err := bazel_testing.RunBazel("build", "@bazel_gazelle_go_repository_config//:all"); err != nil {
		t.Fatal(err)
	}
	stdout, err := bazel_testing.BazelOutput("mod", "dump_repo_mapping", "")
	if err != nil {
		t.Fatal(err)
	}
	var mapping map[string]string
	if err := json.Unmarshal(stdout, &mapping); err != nil {
		t.Fatalf("unmarshaling repo mapping: %v", err)
	}
	configRepoName := mapping["bazel_gazelle_go_repository_config"]
	if configRepoName == "" {
		t.Fatal("repo mapping did not contain bazel_gazelle_go_repository_config")
	}
	outputBase, err := getBazelOutputBase()
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(outputBase, "external", configRepoName)
	testtools.CheckFiles(t, outDir, []testtools.FileSpec{
		{
			Path: "WORKSPACE",
			Content: `
go_repository(
    name = "@gazelle+",
    importpath = "github.com/bazelbuild/bazel-gazelle",
    module_name = "gazelle",
)
go_repository(
    name = "@rules_go+",
    importpath = "github.com/bazelbuild/rules_go",
    module_name = "rules_go",
)
go_repository(
    name = "com_github_apex_log",
    importpath = "github.com/apex/log",
)
go_repository(
    name = "com_github_bazelbuild_buildtools",
    importpath = "github.com/bazelbuild/buildtools",
)
go_repository(
    name = "com_github_bmatcuk_doublestar_v4",
    importpath = "github.com/bmatcuk/doublestar/v4",
)
go_repository(
    name = "com_github_fsnotify_fsnotify",
    importpath = "github.com/fsnotify/fsnotify",
)
go_repository(
    name = "com_github_gogo_protobuf",
    importpath = "github.com/gogo/protobuf",
)
go_repository(
    name = "com_github_golang_mock",
    importpath = "github.com/golang/mock",
)
go_repository(
    name = "com_github_golang_protobuf",
    importpath = "github.com/golang/protobuf",
)
go_repository(
    name = "com_github_google_go_cmp",
    importpath = "github.com/google/go-cmp",
)
go_repository(
    name = "com_github_pkg_errors",
    importpath = "github.com/pkg/errors",
)
go_repository(
    name = "com_github_pmezard_go_difflib",
    importpath = "github.com/pmezard/go-difflib",
)
go_repository(
    name = "org_golang_google_genproto",
    importpath = "google.golang.org/genproto",
)
go_repository(
    name = "org_golang_google_genproto_googleapis_rpc",
    importpath = "google.golang.org/genproto/googleapis/rpc",
)
go_repository(
    name = "org_golang_google_grpc",
    importpath = "google.golang.org/grpc",
)
go_repository(
    name = "org_golang_google_grpc_cmd_protoc_gen_go_grpc",
    importpath = "google.golang.org/grpc/cmd/protoc-gen-go-grpc",
)
go_repository(
    name = "org_golang_google_protobuf",
    importpath = "google.golang.org/protobuf",
)
go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
)
go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
)
go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
)
go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
)
go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
)
go_repository(
    name = "org_golang_x_tools",
    importpath = "golang.org/x/tools",
)
go_repository(
    name = "org_golang_x_tools_go_vcs",
    importpath = "golang.org/x/tools/go/vcs",
)
go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
)`,
		},
	})
}

func TestModcacheRW(t *testing.T) {
	if err := bazel_testing.RunBazel("query", "@com_github_pkg_errors//:go_default_library"); err != nil {
		t.Fatal(err)
	}
	out, err := bazel_testing.BazelOutput("info", "output_base")
	if err != nil {
		t.Fatal(err)
	}
	outputBase := strings.TrimSpace(string(out))
	dir := filepath.Join(outputBase, "external/bazel_gazelle_go_repository_cache/pkg/mod/github.com/pkg/errors@v0.8.1")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o200 == 0 {
		t.Fatal("module cache is read-only")
	}
}

func TestRepoCacheContainsGoEnv(t *testing.T) {
	if err := bazel_testing.RunBazel("query", "@com_github_pkg_errors//:go_default_library"); err != nil {
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
	for _, want := range []string{"GOPRIVATE='example.com/m'", "GOSUMDB='off'"} {
		if !strings.Contains(string(gotBytes), want) {
			t.Fatalf("go.env did not contain %s", want)
		}
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
