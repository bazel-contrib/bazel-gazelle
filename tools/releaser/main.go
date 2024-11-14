/* Copyright 2023 The Bazel Authors. All rights reserved.

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

// releaser is a tool for managing part of the process to release a new version of gazelle.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stderr *os.File) error {
	var (
		verbose   bool
		goVersion string
		repoRoot  string
	)

	flag.BoolVar(&verbose, "verbose", false, "increase verbosity")
	flag.BoolVar(&verbose, "v", false, "increase verbosity (shorthand)")
	flag.StringVar(&goVersion, "go_version", "", "go version for go.mod")
	flag.StringVar(&repoRoot, "repo_root", os.Getenv("BUILD_WORKSPACE_DIRECTORY"), "root directory of Gazelle repo")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), `usage: bazel run //tools/releaser -- -go_version <version>

This utility is intended to handle many of the steps to release a new version.

`)
		flag.PrintDefaults()
	}

	flag.Parse()

	var goVersionArgs []string
	if goVersion != "" {
		versionParts := strings.Split(goVersion, ".")
		if len(versionParts) < 2 {
			flag.Usage()
			return errors.New("please provide a valid Go version")
		}
		if minorVersion, err := strconv.Atoi(versionParts[1]); err != nil {
			return fmt.Errorf("%q is not a valid Go version", goVersion)
		} else if minorVersion > 0 {
			versionParts[1] = strconv.Itoa(minorVersion - 1)
		}
		goVersionArgs = append(goVersionArgs, "-go", goVersion, "-compat", strings.Join(versionParts, "."))
	}

	workspacePath := path.Join(repoRoot, "WORKSPACE")
	depsPath := path.Join(repoRoot, "deps.bzl")
	_tmpBzl := "tmp.bzl"
	tmpBzlPath := path.Join(repoRoot, _tmpBzl)

	if verbose {
		fmt.Println("Running initial go update commands")
	}
	initialCommands := []struct {
		cmd  string
		args []string
	}{
		{cmd: "go", args: append([]string{"mod", "tidy"}, goVersionArgs...)},
		{cmd: "go", args: []string{"mod", "vendor"}},
		{cmd: "find", args: []string{"vendor", "-name", "BUILD.bazel", "-delete"}},
	}
	for _, c := range initialCommands {
		cmd := exec.CommandContext(ctx, c.cmd, c.args...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Println(string(out))
			return err
		}
	}

	workspace, err := os.OpenFile(workspacePath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer workspace.Close()

	if verbose {
		fmt.Println("Preparing temporary WORKSPACE without gazelle directives.")
	}
	workspaceWithoutDirectives, err := getWorkspaceWithoutDirectives(workspace)
	if err != nil {
		return err
	}

	// reuse the open workspace file, so first we empty it and rewind
	err = workspace.Truncate(0)
	if err != nil {
		return err
	}
	_ /* new offset */, err = workspace.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}

	// write the directive-less workspace and update repos
	if _, err := workspace.Write(workspaceWithoutDirectives); err != nil {
		return err
	}

	if verbose {
		fmt.Println("Running update-repos outputting to temporary file.")
	}
	cmd := exec.CommandContext(ctx, "bazel", "run", "//:gazelle", "--", "update-repos", "-from_file=go.mod", fmt.Sprintf("-to_macro=%s%%gazelle_dependencies", _tmpBzl))
	cmd.Dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return err
	}
	defer os.Remove(tmpBzlPath)

	// parse the resulting tmp.bzl for deps.bzl and WORKSPACE updates
	if verbose {
		fmt.Println("Parsing temporary bzl file to prepare deps.bzl and WORKSPACE modifications.")
	}
	maybeRules, workspaceDirectives, err := readFromTmp(tmpBzlPath)
	if err != nil {
		return err
	}

	// update deps
	if verbose {
		fmt.Println("Writing new deps.bzl")
	}
	if err := updateDepsBzlWithRules(depsPath, maybeRules); err != nil {
		return err
	}

	// append WORKSPACE with directives at the end.
	// except we cannot append directly because the earlier bazel //:gazelle run modified WORKSPACE
	// so we truncate and seek to the beginning again before writing all of what we want
	if verbose {
		fmt.Println("Append WORKSPACE with directives")
	}
	_ /* new offset */, err = workspace.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}

	// write the directive-less workspace and update repos
	if _, err := workspace.Write(workspaceWithoutDirectives); err != nil {
		return err
	}
	if _, err := workspace.Write(workspaceDirectives); err != nil {
		return err
	}

	// cleanup before final gazelle run
	//
	// note that we also have a defer for os.Remove so it gets cleaned up if there are earlier errors.
	// This defer will throw an error from this point on, but we're swallowing it anyways.
	if verbose {
		fmt.Println("Cleaning up temporary files")
	}
	if err := os.Remove(tmpBzlPath); err != nil {
		return err
	}

	if verbose {
		fmt.Println("Running final gazelle run, and copying some language specific build files.")
	}
	cmd = exec.CommandContext(ctx, "bazel", "run", "//:gazelle")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return err
	}

	cmd = exec.CommandContext(ctx, "bazel", "build",
		"//language/go:std_package_list",
		"//language/proto:known_go_imports",
		"//language/proto:known_imports",
		"//language/proto:known_proto_imports",
	)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return err
	}

	generatedFiles := []string{
		"language/go/std_package_list.go",
		"language/proto/known_go_imports.go",
		"language/proto/known_imports.go",
		"language/proto/known_proto_imports.go",
	}
	for _, f := range generatedFiles {
		if err := updateFile(repoRoot, f); err != nil {
			return err
		}
	}

	if verbose {
		fmt.Println("Release prepared.")
	}
	return nil
}

func updateFile(repoRoot, filePath string) error {
	destPath := path.Join(repoRoot, filePath)
	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	srcPath := path.Join(repoRoot, "bazel-bin", filePath)
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, src)
	return err
}

func getWorkspaceWithoutDirectives(workspace io.Reader) ([]byte, error) {
	workspaceScanner := bufio.NewScanner(workspace)
	var workspaceWithoutDirectives bytes.Buffer
	for workspaceScanner.Scan() {
		currentLine := workspaceScanner.Text()
		if strings.HasPrefix(currentLine, "# gazelle:repository go_repository") {
			continue
		}
		_, err := workspaceWithoutDirectives.WriteString(currentLine + "\n")
		if err != nil {
			return nil, err
		}
	}
	// leave some buffering at the end of the bytes
	_, err := workspaceWithoutDirectives.WriteString("\n\n")
	if err != nil {
		return nil, err
	}
	return workspaceWithoutDirectives.Bytes(), workspaceScanner.Err()
}

func readFromTmp(tmpBzlPath string) ([]*rule.Rule, []byte, error) {
	workspaceDirectivesBuff := new(bytes.Buffer)
	var rules []*rule.Rule
	tmpBzl, err := rule.LoadMacroFile(tmpBzlPath, "tmp" /* pkg */, "gazelle_dependencies" /* DefName */)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range tmpBzl.Rules {
		maybeRule := rule.NewRule("_maybe", r.Name())
		maybeRule.AddArg(&bzl.Ident{
			Name: r.Kind(),
		})

		for _, k := range r.AttrKeys() {
			maybeRule.SetAttr(k, r.Attr(k))
		}

		var suffix string
		rules = append(rules, maybeRule)
		fmt.Fprintf(workspaceDirectivesBuff, "# gazelle:repository go_repository name=%s importpath=%s%s\n",
			r.Name(),
			r.AttrString("importpath"),
			suffix,
		)
	}
	return rules, workspaceDirectivesBuff.Bytes(), nil
}

func updateDepsBzlWithRules(depsPath string, maybeRules []*rule.Rule) error {
	depsBzl, err := rule.LoadMacroFile(depsPath, "deps" /* pkg */, "gazelle_dependencies" /* DefName */)
	if err != nil {
		return err
	}

	for _, r := range depsBzl.Rules {
		if r.Kind() == "_maybe" && len(r.Args()) == 1 {
			// We can't actually delete all _maybe's because http_archive uses it too in here!
			if ident, ok := r.Args()[0].(*bzl.Ident); ok && ident.Name == "go_repository" {
				r.Delete()
			}
		}
	}

	for _, r := range maybeRules {
		r.Insert(depsBzl)
	}

	return depsBzl.Save(depsPath)
}
