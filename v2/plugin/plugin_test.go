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

package plugin_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bazel-contrib/bazel-gazelle/v2/cmd/gazelle/update"
	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/examples/sh"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/protocol"
	"github.com/bazel-contrib/bazel-gazelle/v2/plugin/server"
	"github.com/bazel-contrib/bazel-gazelle/v2/testtools"
)

// pluginEnv is set in the environment of plugin processes started by these
// tests. The test binary itself acts as the plugin: when pluginEnv is set,
// TestMain serves the plugin named by the first argument (default "sh")
// instead of running tests.
const pluginEnv = "GAZELLE_PLUGIN_TEST_SERVE"

func TestMain(m *testing.M) {
	if os.Getenv(pluginEnv) == "" {
		os.Exit(m.Run())
	}
	mode := "sh"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "sh":
		server.Main(sh.New())
	case "fake":
		server.Main(&fakePlugin{})
	case "badversion":
		server.Main(&badVersionPlugin{})
	case "crash":
		server.Main(&crashPlugin{})
	case "silent":
		server.Main(&silentPlugin{})
	default:
		fmt.Fprintf(os.Stderr, "unknown test plugin %q\n", mode)
		os.Exit(2)
	}
}

// testExecutable returns the path to the test binary and arranges for it to
// act as a plugin when Gazelle starts it.
func testExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(pluginEnv, "1")
	return exe
}

func runGazelle(t *testing.T, dir string, langs []language.Language, args ...string) error {
	t.Helper()
	return update.Run(context.Background(), langs, dir, args)
}

// shRepo is a small repository with shell scripts. The sh plugin (in Go or
// Python) should produce shRepoWant from it.
var shRepo = []testtools.FileSpec{
	{Path: "WORKSPACE"},
	{Path: "BUILD.bazel"},
	{Path: "lib/util.sh", Content: "util() { echo util; }\n"},
	{
		Path: "bin/run.sh",
		Content: `#!/bin/bash
source lib/util.sh
. ./helpers.sh
source "$HOME/.profile"
`,
	},
	{Path: "bin/helpers.sh", Content: "helper() { :; }\n"},
	{
		Path: "bin/run_test.sh",
		Content: `source lib/util.sh
source lib/missing.sh
`,
	},
	{Path: "off/BUILD.bazel", Content: "# gazelle:sh_enabled false\n"},
	{Path: "off/skip.sh", Content: "echo skip\n"},
	{
		Path: "stale/BUILD.bazel",
		Content: `load("@rules_shell//shell:sh_library.bzl", "sh_library")

sh_library(
    name = "gone",
    srcs = ["gone.sh"],
)
`,
	},
	{Path: "stale/kept.sh", Content: "kept() { :; }\n"},
}

var shRepoWant = []testtools.FileSpec{
	{
		Path: "lib/BUILD.bazel",
		Content: `load("@rules_shell//shell:sh_library.bzl", "sh_library")

sh_library(
    name = "util",
    srcs = ["util.sh"],
    visibility = ["//visibility:public"],
)
`,
	},
	{
		Path: "bin/BUILD.bazel",
		Content: `load("@rules_shell//shell:sh_binary.bzl", "sh_binary")
load("@rules_shell//shell:sh_library.bzl", "sh_library")
load("@rules_shell//shell:sh_test.bzl", "sh_test")

sh_library(
    name = "helpers",
    srcs = ["helpers.sh"],
    visibility = ["//visibility:public"],
)

sh_binary(
    name = "run",
    srcs = ["run.sh"],
    deps = [
        ":helpers",
        "//lib:util",
    ],
)

sh_test(
    name = "run_test",
    srcs = ["run_test.sh"],
    deps = ["//lib:util"],
)
`,
	},
	{Path: "off/BUILD.bazel", Content: "# gazelle:sh_enabled false\n"},
	{
		Path: "stale/BUILD.bazel",
		Content: `load("@rules_shell//shell:sh_library.bzl", "sh_library")

sh_library(
    name = "kept",
    srcs = ["kept.sh"],
    visibility = ["//visibility:public"],
)
`,
	},
}

func TestGoReferencePlugin(t *testing.T) {
	exe := testExecutable(t)
	dir, cleanup := testtools.CreateFiles(t, shRepo)
	defer cleanup()
	if err := runGazelle(t, dir, nil, "-plugin="+exe); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, shRepoWant)
}

func TestResolveOverride(t *testing.T) {
	exe := testExecutable(t)
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "BUILD.bazel", Content: "# gazelle:resolve sh vendor/lib.sh //third_party/shlib\n"},
		{Path: "app/main.sh", Content: "#!/bin/sh\nsource vendor/lib.sh\n"},
	})
	defer cleanup()
	if err := runGazelle(t, dir, nil, "-plugin="+exe); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{{
		Path: "app/BUILD.bazel",
		Content: `load("@rules_shell//shell:sh_binary.bzl", "sh_binary")

sh_binary(
    name = "main",
    srcs = ["main.sh"],
    deps = ["//third_party/shlib"],
)
`,
	}})
}

func TestPythonReferencePlugin(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}
	script, err := filepath.Abs(filepath.FromSlash("examples/sh/sh_plugin.py"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("reference plugin not available: %v", err)
	}
	dir, cleanup := testtools.CreateFiles(t, shRepo)
	defer cleanup()
	if err := runGazelle(t, dir, nil, "-plugin="+script); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, shRepoWant)
}

func TestPluginDirective(t *testing.T) {
	exe := testExecutable(t)
	files := append([]testtools.FileSpec{}, shRepo...)
	files[1] = testtools.FileSpec{Path: "BUILD.bazel", Content: "# gazelle:plugin " + filepath.ToSlash(exe) + " sh\n"}
	dir, cleanup := testtools.CreateFiles(t, files)
	defer cleanup()
	if err := runGazelle(t, dir, nil); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, shRepoWant)
}

func TestPluginDirectiveOutsideRoot(t *testing.T) {
	exe := testExecutable(t)
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "sub/BUILD.bazel", Content: "# gazelle:plugin " + filepath.ToSlash(exe) + " sh\n"},
	})
	defer cleanup()
	err := runGazelle(t, dir, nil, "-strict")
	if !errors.Is(err, update.ExitError) {
		t.Fatalf("got error %v; want %v", err, update.ExitError)
	}
}

func TestFixFindLifecycle(t *testing.T) {
	exe := filepath.ToSlash(testExecutable(t))
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "BUILD.bazel", Content: "# gazelle:plugin " + exe + " sh\n# gazelle:plugin " + exe + " fake\n"},
		{
			Path: "fix/BUILD.bazel",
			Content: `old_fake_library(name = "a")

fake_obsolete(name = "b")
`,
		},
		{Path: "app/main.sh", Content: "#!/bin/sh\nsource ext/thing.sh\n"},
	})
	defer cleanup()
	if err := runGazelle(t, dir, nil, "fix"); err != nil {
		t.Fatal(err)
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{
		{
			Path: "fix/BUILD.bazel",
			Content: `load("//tools:fake.bzl", "fake_library")

fake_library(name = "a")
`,
		},
		{
			Path: "app/BUILD.bazel",
			Content: `load("@rules_shell//shell:sh_binary.bzl", "sh_binary")

sh_binary(
    name = "main",
    srcs = ["main.sh"],
    deps = ["@ext//:thing"],
)
`,
		},
		{Path: "fake_events", Content: "onResolve\nonFinish\n"},
	})
}

func TestStartErrors(t *testing.T) {
	exe := filepath.ToSlash(testExecutable(t))
	for _, tc := range []struct {
		desc, rootBuild string
		args            []string
		langs           []language.Language
		wantErr         string
	}{
		{
			desc:    "missing_executable",
			args:    []string{"-plugin=does/not/exist"},
			wantErr: "plugin executable not found",
		},
		{
			desc:      "bad_version",
			rootBuild: "# gazelle:plugin " + exe + " badversion\n",
			wantErr:   "unsupported protocol version 99",
		},
		{
			desc:      "duplicate_name",
			rootBuild: "# gazelle:plugin " + exe + " sh\n# gazelle:plugin " + exe + " sh\n",
			wantErr:   `uses the language name "sh", which is already used by plugin`,
		},
		{
			desc:      "builtin_name",
			rootBuild: "# gazelle:plugin " + exe + " sh\n",
			langs:     []language.Language{namedLanguage("sh")},
			wantErr:   `uses the language name "sh", which is already used by a built-in extension`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
				{Path: "WORKSPACE"},
				{Path: "BUILD.bazel", Content: tc.rootBuild},
			})
			defer cleanup()
			err := runGazelle(t, dir, tc.langs, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got error %v; want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestPluginCrash(t *testing.T) {
	exe := filepath.ToSlash(testExecutable(t))
	dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "BUILD.bazel", Content: "# gazelle:plugin " + exe + " crash\n"},
		{Path: "pkg/x.sh"},
	})
	defer cleanup()
	err := runGazelle(t, dir, nil)
	if !errors.Is(err, update.ExitError) {
		t.Fatalf("got error %v; want %v", err, update.ExitError)
	}
	testtools.CheckFiles(t, dir, []testtools.FileSpec{{Path: "pkg/BUILD.bazel", NotExist: true}})
}

func TestRequestCancel(t *testing.T) {
	exe := testExecutable(t)
	c := config.New()
	c.RepoRoot = t.TempDir()
	c.WorkDir = c.RepoRoot
	parentCtx, stopParent := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopParent()
	l, err := plugin.Start(parentCtx, c, plugin.Options{Path: exe, Args: []string{"silent"}})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// An already-canceled request must leave the healthy plugin usable.
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Configure(canceledCtx, config.ConfigureArgs{Config: c}); !errors.Is(err, context.Canceled) {
		t.Fatalf("already-canceled Configure: got %v, want context.Canceled", err)
	}

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	defer cancelRequest()
	done := make(chan error, 1)
	go func() { done <- l.Configure(requestCtx, config.ConfigureArgs{Config: c}) }()
	// Wait until the plugin has read the request and is blocking on its own
	// work. Canceling only the request must interrupt the pending response.
	marker := filepath.Join(c.RepoRoot, "configure_started")
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			t.Fatalf("Configure returned before plugin received request: %v", err)
		case <-parentCtx.Done():
			t.Fatal("plugin did not receive Configure")
		case <-ticker.C:
		}
	}
	cancelRequest()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Configure: got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		stopParent()
		t.Fatal("Configure did not return after request cancellation")
	}
	if err := parentCtx.Err(); err != nil {
		t.Fatalf("request cancellation needed parent cancellation: %v", err)
	}
	closed := make(chan error, 1)
	go func() { closed <- l.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close after request cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		stopParent()
		t.Fatal("Close did not return after request cancellation")
	}
}

type silentPlugin struct {
	repoRoot string
}

func (p *silentPlugin) Initialize(ctx context.Context, params protocol.InitializeParams) (protocol.InitializeResult, error) {
	p.repoRoot = params.RepoRoot
	return protocol.InitializeResult{
		ProtocolVersion: protocol.Version,
		Name:            "silent",
		Capabilities:    protocol.Capabilities{Configure: protocol.ConfigureAll},
	}, nil
}

func (p *silentPlugin) Configure(ctx context.Context, params protocol.ConfigureParams) (protocol.ConfigureResult, error) {
	if err := os.WriteFile(filepath.Join(p.repoRoot, "configure_started"), nil, 0o600); err != nil {
		return protocol.ConfigureResult{}, err
	}
	time.Sleep(time.Minute)
	return protocol.ConfigureResult{}, nil
}

type namedLanguage string

func (n namedLanguage) Name() string { return string(n) }

// fakePlugin tests the optional fix, find, and lifecycle capabilities.
type fakePlugin struct {
	repoRoot string
}

func (p *fakePlugin) Initialize(ctx context.Context, params protocol.InitializeParams) (protocol.InitializeResult, error) {
	p.repoRoot = params.RepoRoot
	return protocol.InitializeResult{
		ProtocolVersion: protocol.Version,
		Name:            "fake",
		Kinds: []protocol.KindInfo{{
			Name:           "fake_library",
			LoadedFrom:     "//tools:fake.bzl",
			MergeableAttrs: []string{"srcs"},
		}},
		Capabilities: protocol.Capabilities{
			Configure: protocol.ConfigureNone,
			Fix:       true,
			Find:      true,
			Lifecycle: true,
		},
	}, nil
}

// Fix renames old_fake_library to fake_library and deletes fake_obsolete.
func (p *fakePlugin) Fix(ctx context.Context, params protocol.FixParams) (protocol.FixResult, error) {
	var res protocol.FixResult
	if !params.ShouldFix {
		return res, nil
	}
	for i, r := range params.File.Rules {
		switch r.Kind {
		case "old_fake_library":
			res.Edits = append(res.Edits, protocol.RuleEdit{Index: i, Kind: "fake_library"})
		case "fake_obsolete":
			res.Edits = append(res.Edits, protocol.RuleEdit{Index: i, Delete: true})
		}
	}
	return res, nil
}

// Find resolves shell scripts under ext/ to an external repository.
func (p *fakePlugin) Find(ctx context.Context, params protocol.FindParams) (protocol.FindResult, error) {
	var res protocol.FindResult
	if params.Import.Lang == "sh" && strings.HasPrefix(params.Import.Imp, "ext/") {
		name := strings.TrimSuffix(path.Base(params.Import.Imp), ".sh")
		res.Results = append(res.Results, protocol.FindMatch{Label: "@ext//:" + name})
	}
	return res, nil
}

func (p *fakePlugin) OnResolve(ctx context.Context) error { return p.recordEvent("onResolve") }

func (p *fakePlugin) OnFinish(ctx context.Context) error { return p.recordEvent("onFinish") }

func (p *fakePlugin) recordEvent(event string) error {
	f, err := os.OpenFile(filepath.Join(p.repoRoot, "fake_events"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, event); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type badVersionPlugin struct{}

func (badVersionPlugin) Initialize(ctx context.Context, params protocol.InitializeParams) (protocol.InitializeResult, error) {
	return protocol.InitializeResult{ProtocolVersion: 99, Name: "bad"}, nil
}

// crashPlugin exits abruptly while generating rules.
type crashPlugin struct{}

func (crashPlugin) Initialize(ctx context.Context, params protocol.InitializeParams) (protocol.InitializeResult, error) {
	return protocol.InitializeResult{ProtocolVersion: protocol.Version, Name: "crash"}, nil
}

func (crashPlugin) Generate(ctx context.Context, params protocol.GenerateParams) (protocol.GenerateResult, error) {
	if len(params.RegularFiles) > 0 {
		os.Exit(3)
	}
	return protocol.GenerateResult{}, nil
}
