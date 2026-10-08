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
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	gzflag "github.com/bazel-contrib/bazel-gazelle/v2/flag"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazelbuild/rules_go/go/runfiles"
)

// pluginDirective is the directive that registers a plugin in the repository
// root build file.
const pluginDirective = "plugin"

// Loader starts the plugins a user registered, either with the -plugin
// command-line flag (which may be repeated) or with
// "# gazelle:plugin path [args...]" directives in the repository root build
// file.
//
// Loader implements [config.Configurer] and the RegisterFlags / CheckFlags
// methods of [github.com/bazel-contrib/bazel-gazelle/v2/compat.FlagConfigurer].
// Gazelle registers its flags with the other built-in configurers, then calls
// [Loader.Start] after flags have been parsed and adds the returned languages
// to the set of extensions for the run.
type Loader struct {
	flagValues       []string
	languages        []*Language
	goRepositoryMode bool
}

var _ config.Configurer = (*Loader)(nil)

// RegisterFlags registers the -plugin flag.
func (ld *Loader) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	fs.Var(&gzflag.MultiFlag{Values: &ld.flagValues}, "plugin", "path to a language plugin executable that Gazelle runs as a subprocess (may be repeated)")
}

// CheckFlags records whether Gazelle is processing a fetched Go repository.
// The Go extension registers this flag; all flags have been parsed by now,
// even when that extension's CheckFlags has not run yet.
func (ld *Loader) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	f := fs.Lookup("go_repository_mode")
	ld.goRepositoryMode = f != nil && f.Value.String() == "true"
	return nil
}

// KnownDirectives returns the plugin directive.
func (ld *Loader) KnownDirectives() []string { return []string{pluginDirective} }

// Configure reports an error if the plugin directive appears anywhere other
// than the repository root build file. The directive itself is read by Start.
func (ld *Loader) Configure(ctx context.Context, args config.ConfigureArgs) error {
	if ld.goRepositoryMode || args.Rel == "" || args.File == nil {
		return nil
	}
	for _, d := range args.File.Directives {
		if d.Key == pluginDirective {
			return fmt.Errorf("%s: gazelle:plugin may only be used in the repository root build file", args.File.Path)
		}
	}
	return nil
}

// spec describes a plugin to start.
type spec struct {
	origin string // for error messages
	path   string
	args   []string
}

// Start starts each registered plugin and returns them in the order they were
// registered: plugins named on the command line first, then plugins named by
// directives. c must have been initialized from command-line flags.
// reservedNames lists the names of extensions compiled into Gazelle; plugin
// names must not collide with them or with each other.
//
// Start returns an error if any plugin can't be started. The caller must call
// Close when the plugins are no longer needed, even if Start fails.
func (ld *Loader) Start(ctx context.Context, c *config.Config, reservedNames []string) ([]*Language, error) {
	specs, err := ld.specs(c)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	env := pluginEnv()
	names := make(map[string]string)
	for _, n := range reservedNames {
		names[n] = "a built-in extension"
	}
	for _, s := range specs {
		lang, err := Start(ctx, c, Options{Path: s.path, Args: s.args, Env: env})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.origin, err)
		}
		ld.languages = append(ld.languages, lang)
		if other, ok := names[lang.Name()]; ok {
			return nil, fmt.Errorf("%s: plugin %s uses the language name %q, which is already used by %s", s.origin, s.path, lang.Name(), other)
		}
		names[lang.Name()] = "plugin " + s.path
	}
	return slices.Clone(ld.languages), nil
}

// Close shuts down all plugins started by Start.
func (ld *Loader) Close() error {
	var errs []error
	for _, l := range ld.languages {
		errs = append(errs, l.Close())
	}
	return errors.Join(errs...)
}

func (ld *Loader) specs(c *config.Config) ([]spec, error) {
	var specs []spec
	for _, v := range ld.flagValues {
		path, err := findFlagPlugin(c, v)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec{origin: "-plugin=" + v, path: path})
	}

	// A fetched dependency's BUILD files must not select executable plugins.
	// Explicit -plugin arguments still come from the invoking workspace.
	if ld.goRepositoryMode {
		return specs, nil
	}

	f, err := loadRootBuildFile(c)
	if err != nil {
		return nil, err
	}
	if f != nil {
		for _, d := range f.Directives {
			if d.Key != pluginDirective {
				continue
			}
			origin := fmt.Sprintf("%s: gazelle:plugin %s", f.Path, d.Value)
			fields := strings.Fields(d.Value)
			if len(fields) == 0 {
				return nil, fmt.Errorf("%s: expected a path to a plugin executable", origin)
			}
			path := filepath.FromSlash(fields[0])
			if !filepath.IsAbs(path) {
				path = filepath.Join(c.RepoRoot, path)
			}
			specs = append(specs, spec{origin: origin, path: path, args: fields[1:]})
		}
	}
	return specs, nil
}

// findFlagPlugin locates a plugin executable named with -plugin. Absolute
// paths are used as is. Relative paths are looked up, in order:
//
//   - in Bazel runfiles, for paths written with $(rlocationpath ...);
//   - relative to the current directory, for paths written with
//     $(rootpath ...) when Gazelle is run with 'bazel run';
//   - relative to the workspace directory (c.WorkDir);
//   - on PATH, for names without a directory separator.
func findFlagPlugin(c *config.Config, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("-plugin: expected a path to a plugin executable")
	}
	p := filepath.FromSlash(value)
	if filepath.IsAbs(p) {
		return p, nil
	}
	if loc, err := runfiles.Rlocation(value); err == nil && loc != "" && isFile(loc) {
		return loc, nil
	}
	if wd, err := os.Getwd(); err == nil {
		if cand := filepath.Join(wd, p); isFile(cand) {
			return cand, nil
		}
	}
	if c.WorkDir != "" {
		if cand := filepath.Join(c.WorkDir, p); isFile(cand) {
			return cand, nil
		}
	}
	if !strings.ContainsAny(value, `/\`) {
		if lp, err := exec.LookPath(value); err == nil {
			return lp, nil
		}
	}
	return "", fmt.Errorf("-plugin=%s: plugin executable not found in runfiles, the current directory, %s, or PATH", value, c.WorkDir)
}

// loadRootBuildFile reads the repository root build file to find plugin
// directives. Gazelle hasn't walked the repository yet when plugins are
// started, since plugins may declare directives and kinds that affect the
// walk. Returns nil if there is no root build file.
func loadRootBuildFile(c *config.Config) (*rule.File, error) {
	dir := c.RepoRoot
	if c.ReadBuildFilesDir != "" {
		dir = c.ReadBuildFilesDir
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	path := rule.MatchBuildFile(dir, c.ValidBuildFileNames, ents)
	if path == "" {
		return nil, nil
	}
	f, err := rule.LoadFile(path, "")
	if err != nil {
		// Gazelle reports syntax errors when it walks the repository.
		return nil, nil
	}
	return f, nil
}

// pluginEnv returns the environment for plugin processes. When Gazelle runs
// with Bazel runfiles (for example, with 'bazel run //:gazelle'), the runfiles
// variables are passed on, so plugins built by Bazel (which are usually
// listed in the data attribute of the gazelle rule) can find their own
// runfiles.
func pluginEnv() []string {
	env := os.Environ()
	if rfEnv, err := runfiles.Env(); err == nil {
		env = append(env, rfEnv...)
	}
	return env
}

func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
