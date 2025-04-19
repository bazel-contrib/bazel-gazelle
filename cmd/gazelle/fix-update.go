/* Copyright 2017 The Bazel Authors. All rights reserved.

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

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/bazelbuild/buildtools/build"

	"github.com/bazelbuild/bazel-gazelle/config"
	gzflag "github.com/bazelbuild/bazel-gazelle/flag"
	"github.com/bazelbuild/bazel-gazelle/internal/wspace"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/walk"
)

// updateConfig holds configuration information needed to run the fix and
// update commands. This includes everything in config.Config, but it also
// includes some additional fields that aren't relevant to other packages.
type updateConfig struct {
	dirs           []string
	emit           emitFunc
	repos          []repo.Repo
	workspaceFiles []*rule.File
	walkMode       walk.Mode
	patchPath      string
	patchBuffer    bytes.Buffer
	print0         bool
	profile        profiler
}

type emitFunc func(c *config.Config, f *rule.File) error

var modeFromName = map[string]emitFunc{
	"print": printFile,
	"fix":   fixFile,
	"diff":  diffFile,
}

const updateName = "_update"

func getUpdateConfig(c *config.Config) *updateConfig {
	return c.Exts[updateName].(*updateConfig)
}

var _ config.Configurer = (*updateConfigurer)(nil)

type updateConfigurer struct {
	mode           string
	recursive      bool
	knownImports   []string
	repoConfigPath string
	cpuProfile     string
	memProfile     string
}

func (ucr *updateConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	uc := &updateConfig{}
	c.Exts[updateName] = uc

	c.ShouldFix = cmd == "fix"

	fs.StringVar(&ucr.mode, "mode", "fix", "print: prints all of the updated BUILD files\n\tfix: rewrites all of the BUILD files in place\n\tdiff: computes the rewrite but then just does a diff")
	fs.BoolVar(&ucr.recursive, "r", true, "when true, gazelle will update subdirectories recursively")
	fs.StringVar(&uc.patchPath, "patch", "", "when set with -mode=diff, gazelle will write to a file instead of stdout")
	fs.BoolVar(&uc.print0, "print0", false, "when set with -mode=fix, gazelle will print the names of rewritten files separated with \\0 (NULL)")
	fs.StringVar(&ucr.cpuProfile, "cpuprofile", "", "write cpu profile to `file`")
	fs.StringVar(&ucr.memProfile, "memprofile", "", "write memory profile to `file`")
	fs.Var(&gzflag.MultiFlag{Values: &ucr.knownImports}, "known_import", "import path for which external resolution is skipped (can specify multiple times)")
	fs.StringVar(&ucr.repoConfigPath, "repo_config", "", "file where Gazelle should load repository configuration. Defaults to WORKSPACE.")
}

func (ucr *updateConfigurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	uc := getUpdateConfig(c)

	var ok bool
	uc.emit, ok = modeFromName[ucr.mode]
	if !ok {
		return fmt.Errorf("unrecognized emit mode: %q", ucr.mode)
	}
	if uc.patchPath != "" && ucr.mode != "diff" {
		return fmt.Errorf("-patch set but -mode is %s, not diff", ucr.mode)
	}
	if uc.patchPath != "" && !filepath.IsAbs(uc.patchPath) {
		uc.patchPath = filepath.Join(c.WorkDir, uc.patchPath)
	}
	p, err := newProfiler(ucr.cpuProfile, ucr.memProfile)
	if err != nil {
		return err
	}
	uc.profile = p

	dirs := fs.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	uc.dirs = make([]string, len(dirs))
	for i, arg := range dirs {
		dir := arg
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(c.WorkDir, dir)
		}
		dir, err = filepath.EvalSymlinks(dir)
		if err != nil {
			return fmt.Errorf("%s: failed to resolve symlinks: %v", arg, err)
		}
		if !isDescendingDir(dir, c.RepoRoot) {
			return fmt.Errorf("%s: not a subdirectory of repo root %s", arg, c.RepoRoot)
		}
		uc.dirs[i] = dir
	}

	indexAll := c.IndexLibraries && !c.IndexLazy
	switch {
	case ucr.recursive && indexAll:
		uc.walkMode = walk.VisitAllUpdateSubdirsMode
	case !ucr.recursive && indexAll:
		uc.walkMode = walk.VisitAllUpdateDirsMode
	case ucr.recursive && !indexAll:
		uc.walkMode = walk.UpdateSubdirsMode
	case !ucr.recursive && !indexAll:
		uc.walkMode = walk.UpdateDirsMode
	}

	// Load the repo configuration file (WORKSPACE by default) to find out
	// names and prefixes of other go_repositories. This affects external
	// dependency resolution for Go.
	// TODO(jayconrod): Go-specific code should be moved to language/go.
	if ucr.repoConfigPath == "" {
		ucr.repoConfigPath = wspace.FindWORKSPACEFile(c.RepoRoot)
	}
	repoConfigFile, err := rule.LoadWorkspaceFile(ucr.repoConfigPath, "")
	if err != nil && !os.IsNotExist(err) && !isDirErr(err) {
		return err
	} else if err == nil {
		c.Repos, _, err = repo.ListRepositories(repoConfigFile)
		if err != nil {
			return err
		}
	}
	for _, imp := range ucr.knownImports {
		uc.repos = append(uc.repos, repo.Repo{
			Name:     label.ImportPathToBazelRepoName(imp),
			GoPrefix: imp,
		})
	}

	for _, r := range c.Repos {
		if r.Kind() == "go_repository" {
			var name string
			if apparentName := c.ModuleToApparentName(r.AttrString("module_name")); apparentName != "" {
				name = apparentName
			} else {
				name = r.Name()
			}
			uc.repos = append(uc.repos, repo.Repo{
				Name:     name,
				GoPrefix: r.AttrString("importpath"),
			})
		}
	}

	// If the repo configuration file is not WORKSPACE, also load WORKSPACE
	// and any declared macro files so we can apply fixes.
	workspacePath := wspace.FindWORKSPACEFile(c.RepoRoot)
	var workspace *rule.File
	if ucr.repoConfigPath == workspacePath {
		workspace = repoConfigFile
	} else {
		workspace, err = rule.LoadWorkspaceFile(workspacePath, "")
		if err != nil && !os.IsNotExist(err) && !isDirErr(err) {
			return err
		}
	}
	if workspace != nil {
		c.RepoName = findWorkspaceName(workspace)
		_, repoFileMap, err := repo.ListRepositories(workspace)
		if err != nil {
			return err
		}
		seen := make(map[*rule.File]bool)
		for _, f := range repoFileMap {
			if !seen[f] {
				uc.workspaceFiles = append(uc.workspaceFiles, f)
				seen[f] = true
			}
		}
		sort.Slice(uc.workspaceFiles, func(i, j int) bool {
			return uc.workspaceFiles[i].Path < uc.workspaceFiles[j].Path
		})
	}

	return nil
}

func (ucr *updateConfigurer) KnownDirectives() []string { return nil }

func (ucr *updateConfigurer) Configure(c *config.Config, rel string, f *rule.File) {}

// visitRecord stores information about a directory visited with
// packages.Walk.
type visitRecord struct {
	// pkgRel is the slash-separated path to the visited directory, relative to
	// the repository root. "" for the repository root itself.
	pkgRel string

	// c is the configuration for the directory with directives applied.
	c *config.Config

	// rules is a list of generated Go rules.
	rules []*rule.Rule

	// imports contains opaque import information for each rule in rules.
	imports []interface{}

	// empty is a list of empty Go rules that may be deleted.
	empty []*rule.Rule

	// file is the build file being processed.
	file *rule.File

	// mappedKinds are mapped kinds used during this visit.
	mappedKinds    []config.MappedKind
	mappedKindInfo map[string]rule.KindInfo
}

var genericLoads = []rule.LoadInfo{
	{
		Name:    "@bazel_gazelle//:def.bzl",
		Symbols: []string{"gazelle"},
	},
}

func runFixUpdate(wd string, cmd command, args []string) (err error) {
	cexts := make([]config.Configurer, 0, len(languages)+4)
	cexts = append(cexts,
		&config.CommonConfigurer{},
		&updateConfigurer{},
		&walk.Configurer{},
		&resolve.Configurer{})

	for _, lang := range languages {
		cexts = append(cexts, lang)
	}

	c, err := newFixUpdateConfiguration(wd, cmd, args, cexts)
	if err != nil {
		return err
	}

	mrslv := newMetaResolver()
	kinds := make(map[string]rule.KindInfo)
	loads := genericLoads
	exts := make([]interface{}, 0, len(languages))
	for _, lang := range languages {
		for kind, info := range lang.Kinds() {
			mrslv.AddBuiltin(kind, lang)
			kinds[kind] = info
		}
		if moduleAwareLang, ok := lang.(language.ModuleAwareLanguage); ok {
			loads = append(loads, moduleAwareLang.ApparentLoads(c.ModuleToApparentName)...)
		} else {
			loads = append(loads, lang.Loads()...)
		}
		exts = append(exts, lang)
	}
	ruleIndex := resolve.NewRuleIndex(mrslv.Resolver, exts...)

	if err = fixRepoFiles(c, loads); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, lang := range languages {
		if life, ok := lang.(language.LifecycleManager); ok {
			life.Before(ctx)
		}
	}

	// Visit all directories in the repository.
	var visits []visitRecord
	uc := getUpdateConfig(c)
	defer func() {
		if err := uc.profile.stop(); err != nil {
			log.Printf("stopping profiler: %v", err)
		}
	}()

	walkErr := walk.Walk2(c, cexts, uc.dirs, uc.walkMode, func(args walk.Walk2FuncArgs) walk.Walk2FuncResult {
		dir := args.Dir
		rel := args.Rel
		c := args.Config
		update := args.Update
		f := args.File
		subdirs := args.Subdirs
		regularFiles := args.RegularFiles
		genFiles := args.GenFiles

		mrslv.AliasedKinds(rel, c.AliasMap)
		// If this file is ignored or if Gazelle was not asked to update this
		// directory, just index the build file and move on.
		if !update {
			for _, repl := range c.KindMap {
				mrslv.MappedKind(rel, repl)
			}
			if c.IndexLibraries && f != nil {
				for _, r := range f.Rules {
					ruleIndex.AddRule(c, r, f)
				}
			}
			return walk.Walk2FuncResult{}
		}

		// Fix any problems in the file.
		if f != nil {
			for _, l := range filterLanguages(c, languages) {
				l.Fix(c, f)
			}
		}

		// Generate rules.
		var empty, gen []*rule.Rule
		var imports []interface{}
		var relsToVisit []string
		for _, l := range filterLanguages(c, languages) {
			res := l.GenerateRules(language.GenerateArgs{
				Config:       c,
				Dir:          dir,
				Rel:          rel,
				File:         f,
				Subdirs:      subdirs,
				RegularFiles: regularFiles,
				GenFiles:     genFiles,
				OtherEmpty:   empty,
				OtherGen:     gen,
			})
			if len(res.Gen) != len(res.Imports) {
				log.Panicf("%s: language %s generated %d rules but returned %d imports", rel, l.Name(), len(res.Gen), len(res.Imports))
			}
			empty = append(empty, res.Empty...)
			gen = append(gen, res.Gen...)
			imports = append(imports, res.Imports...)
			if c.IndexLibraries {
				relsToVisit = append(relsToVisit, res.RelsToIndex...)
			}
		}
		if f == nil && len(gen) == 0 {
			return walk.Walk2FuncResult{RelsToVisit: relsToVisit}
		}

		// Apply and record relevant kind mappings.
		var (
			mappedKinds    []config.MappedKind
			mappedKindInfo = make(map[string]rule.KindInfo)
		)
		// We apply map_kind to all rules, including pre-existing ones.
		var allRules []*rule.Rule
		allRules = append(allRules, gen...)
		if f != nil {
			allRules = append(allRules, f.Rules...)
		}

		maybeRecordReplacement := func(ruleKind string) (*string, error) {
			var repl *config.MappedKind
			repl, err = lookupMapKindReplacement(c.KindMap, ruleKind)
			if err != nil {
				return nil, err
			}
			if repl != nil {
				mappedKindInfo[repl.KindName] = kinds[ruleKind]
				mappedKinds = append(mappedKinds, *repl)
				mrslv.MappedKind(rel, *repl)
				return &repl.KindName, nil
			}
			return nil, nil
		}

		var errs []error
		for _, r := range allRules {
			if replacementName, err := maybeRecordReplacement(r.Kind()); err != nil {
				errs = append(errs, fmt.Errorf("looking up mapped kind: %w", err))
			} else if replacementName != nil {
				r.SetKind(*replacementName)
			}

			for i, arg := range r.Args() {
				// Only check the first arg - this supports the maybe(java_library, ...) pattern,
				// but avoids potential false positives from other uses of symbols.
				if i != 0 {
					break
				}
				if ident, ok := arg.(*build.Ident); ok {
					// Don't allow re-mapping symbols that aren't known loads of a plugin.
					if _, knownKind := kinds[ident.Name]; !knownKind {
						continue
					}
					if replacementName, err := maybeRecordReplacement(ident.Name); err != nil {
						errs = append(errs, fmt.Errorf("looking up mapped kind: %w", err))
					} else if replacementName != nil {
						if err := r.UpdateArg(i, &build.Ident{Name: *replacementName}); err != nil {
							log.Panicf("%s: %v", rel, err)
						}
					}
				}
			}
		}
		for _, r := range empty {
			if repl, ok := c.KindMap[r.Kind()]; ok {
				mappedKindInfo[repl.KindName] = kinds[r.Kind()]
				mappedKinds = append(mappedKinds, repl)
				mrslv.MappedKind(rel, repl)
				r.SetKind(repl.KindName)
			}
		}

		// Insert or merge rules into the build file.
		if f == nil {
			f = rule.EmptyFile(filepath.Join(dir, c.DefaultBuildFileName()), rel)
			for _, r := range gen {
				r.Insert(f)
			}
		} else {
			merger.MergeFile(f, empty, gen, merger.PreResolve,
				unionKindInfoMaps(kinds, mappedKindInfo),
				c.AliasMap,
			)
		}
		visits = append(visits, visitRecord{
			pkgRel:         rel,
			c:              c,
			rules:          gen,
			imports:        imports,
			empty:          empty,
			file:           f,
			mappedKinds:    mappedKinds,
			mappedKindInfo: mappedKindInfo,
		})

		// Add library rules to the dependency resolution table.
		if c.IndexLibraries {
			for _, r := range f.Rules {
				ruleIndex.AddRule(c, r, f)
			}
		}

		return walk.Walk2FuncResult{
			RelsToVisit: relsToVisit,
			Err:         errors.Join(errs...),
		}
	})

	for _, lang := range languages {
		if finishable, ok := lang.(language.FinishableLanguage); ok {
			finishable.DoneGeneratingRules()
		}
	}

	if walkErr != nil {
		return walkErr
	}

	// Finish building the index for dependency resolution.
	ruleIndex.Finish()

	// Resolve dependencies.
	rc, cleanupRc := repo.NewRemoteCache(uc.repos)
	defer func() {
		if cerr := cleanupRc(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	if err = maybePopulateRemoteCacheFromGoMod(c, rc); err != nil {
		log.Print(err)
	}
	for _, v := range visits {
		for i, r := range v.rules {
			from := label.New(c.RepoName, v.pkgRel, r.Name())
			if rslv := mrslv.Resolver(r, v.pkgRel); rslv != nil {
				rslv.Resolve(v.c, ruleIndex, rc, r, v.imports[i], from)
			}
		}
		merger.MergeFile(v.file, v.empty, v.rules, merger.PostResolve,
			unionKindInfoMaps(kinds, v.mappedKindInfo),
			v.c.AliasMap,
		)
	}
	for _, lang := range languages {
		if life, ok := lang.(language.LifecycleManager); ok {
			life.AfterResolvingDeps(ctx)
		}
	}

	// Emit merged files.
	var exit error
	for _, v := range visits {
		merger.FixLoads(v.file, applyKindMappings(v.mappedKinds, loads))
		if err := uc.emit(v.c, v.file); err != nil {
			if err == errExit {
				exit = err
			} else {
				log.Print(err)
			}
		}
	}
	if uc.patchPath != "" {
		if err := os.WriteFile(uc.patchPath, uc.patchBuffer.Bytes(), 0o666); err != nil {
			return err
		}
	}

	return exit
}

// lookupMapKindReplacement finds a mapped replacement for rule kind `kind`, resolving transitively.
// i.e. if go_library is mapped to custom_go_library, and custom_go_library is mapped to other_go_library,
// looking up go_library will return other_go_library.
// It returns an error on a loop, and may return nil if no remapping should be performed.
func lookupMapKindReplacement(kindMap map[string]config.MappedKind, kind string) (*config.MappedKind, error) {
	var mapped *config.MappedKind
	seenKinds := make(map[string]struct{})
	seenKindPath := []string{kind}
	for {
		replacement, ok := kindMap[kind]
		if !ok {
			break
		}

		seenKindPath = append(seenKindPath, replacement.KindName)
		if _, alreadySeen := seenKinds[replacement.KindName]; alreadySeen {
			return nil, fmt.Errorf("found loop of map_kind replacements: %s", strings.Join(seenKindPath, " -> "))
		}

		seenKinds[replacement.KindName] = struct{}{}
		mapped = &replacement
		if kind == replacement.KindName {
			break
		}

		kind = replacement.KindName
	}

	return mapped, nil
}

func newFixUpdateConfiguration(wd string, cmd command, args []string, cexts []config.Configurer) (*config.Config, error) {
	c := config.New()
	c.WorkDir = wd

	fs := flag.NewFlagSet("gazelle", flag.ContinueOnError)
	// Flag will call this on any parse error. Don't print usage unless
	// -h or -help were passed explicitly.
	fs.Usage = func() {}

	for _, cext := range cexts {
		cext.RegisterFlags(fs, cmd.String(), c)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fixUpdateUsage(fs)
			return nil, err
		}
		// flag already prints the error; don't print it again.
		log.Fatal("Try -help for more information.")
	}

	for _, cext := range cexts {
		if err := cext.CheckFlags(fs, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func fixUpdateUsage(fs *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `usage: gazelle [fix|update] [flags...] [package-dirs...]

The update command creates new build files and update existing BUILD files
when needed.

The fix command also creates and updates build files, and in addition, it may
make potentially breaking updates to usage of rules. For example, it may
delete obsolete rules or rename existing rules.

There are several output modes which can be selected with the -mode flag. The
output mode determines what Gazelle does with updated BUILD files.

  fix (default) - write updated BUILD files back to disk.
  print - print updated BUILD files to stdout.
  diff - diff updated BUILD files against existing files in unified format.

Gazelle accepts a list of paths to Go package directories to process (defaults
to the working directory if none are given). It recursively traverses
subdirectories. All directories must be under the directory specified by
-repo_root; if -repo_root is not given, this is the directory containing the
WORKSPACE file.

FLAGS:

`)
	fs.PrintDefaults()
}

func fixRepoFiles(c *config.Config, loads []rule.LoadInfo) error {
	uc := getUpdateConfig(c)
	if !c.ShouldFix {
		return nil
	}
	shouldFix := false
	for _, d := range uc.dirs {
		if d == c.RepoRoot {
			shouldFix = true
			break
		}
	}
	if !shouldFix {
		return nil
	}

	for _, f := range uc.workspaceFiles {
		merger.FixLoads(f, loads)
		workspaceFile := wspace.FindWORKSPACEFile(c.RepoRoot)

		if f.Path == workspaceFile {
			if err := merger.CheckGazelleLoaded(f); err != nil {
				return err
			}
		}
		if err := uc.emit(c, f); err != nil {
			return err
		}
	}
	return nil
}

func findWorkspaceName(f *rule.File) string {
	var name string
	for _, r := range f.Rules {
		if r.Kind() == "workspace" {
			name = r.Name()
			break
		}
	}
	// HACK(bazelbuild/rules_go#2355, bazelbuild/rules_go#2387):
	// We can't patch the WORKSPACE file with the correct name because Bazel
	// writes it first; our patches won't apply.
	if name == "com_google_googleapis" {
		return "go_googleapis"
	}
	return name
}

func isDescendingDir(dir, root string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

func findOutputPath(c *config.Config, f *rule.File) string {
	if c.ReadBuildFilesDir == "" && c.WriteBuildFilesDir == "" {
		return f.Path
	}
	baseDir := c.WriteBuildFilesDir
	if c.WriteBuildFilesDir == "" {
		baseDir = c.RepoRoot
	}
	outputDir := filepath.Join(baseDir, filepath.FromSlash(f.Pkg))
	defaultOutputPath := filepath.Join(outputDir, c.DefaultBuildFileName())
	ents, err := os.ReadDir(outputDir)
	if err != nil {
		// Ignore error. Directory probably doesn't exist.
		return defaultOutputPath
	}
	outputPath := rule.MatchBuildFile(outputDir, c.ValidBuildFileNames, ents)
	if outputPath == "" {
		return defaultOutputPath
	}
	return outputPath
}

// maybePopulateRemoteCacheFromGoMod reads go.mod and adds a root to rc for each
// module requirement. This lets the Go extension avoid a network lookup for
// unknown imports with -external=external, and it lets dependency resolution
// succeed with -external=static when it might not otherwise.
//
// This function does not override roots added from WORKSPACE (or some other
// configuration file), but it's useful when there is no such file. In most
// cases, a user of Gazelle with indirect Go dependencies does not need to add
// '# gazelle:repository' or '# gazelle:repository_macro' directives to their
// WORKSPACE file. This need was frustrating for developers in non-Go
// repositories with go_repository dependencies declared in macros. It wasn't
// obvious that Gazelle was missing these.
//
// This function is regrettably Go specific and does not belong here, but it
// can't be moved to //language/go until //repo is broken up and moved there.
func maybePopulateRemoteCacheFromGoMod(c *config.Config, rc *repo.RemoteCache) error {
	haveGo := false
	for name := range c.Exts {
		if name == "go" {
			haveGo = true
			break
		}
	}
	if !haveGo {
		return nil
	}

	goModPath := filepath.Join(c.RepoRoot, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil
	}

	return rc.PopulateFromGoMod(goModPath)
}

func unionKindInfoMaps(a, b map[string]rule.KindInfo) map[string]rule.KindInfo {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	result := make(map[string]rule.KindInfo, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

// applyKindMappings returns a copy of LoadInfo that includes c.KindMap.
func applyKindMappings(mappedKinds []config.MappedKind, loads []rule.LoadInfo) []rule.LoadInfo {
	if len(mappedKinds) == 0 {
		return loads
	}

	// Add new RuleInfos or replace existing ones with merged ones.
	mappedLoads := make([]rule.LoadInfo, len(loads))
	copy(mappedLoads, loads)
	for _, mappedKind := range mappedKinds {
		mappedLoads = appendOrMergeKindMapping(mappedLoads, mappedKind)
	}
	return mappedLoads
}

// appendOrMergeKindMapping adds LoadInfo for the given replacement.
func appendOrMergeKindMapping(mappedLoads []rule.LoadInfo, mappedKind config.MappedKind) []rule.LoadInfo {
	// If mappedKind.KindLoad already exists in the list, create a merged copy.
	for i, load := range mappedLoads {
		if load.Name == mappedKind.KindLoad {
			mappedLoads[i].Symbols = append(load.Symbols, mappedKind.KindName)
			return mappedLoads
		}
	}

	// Add a new LoadInfo.
	return append(mappedLoads, rule.LoadInfo{
		Name:    mappedKind.KindLoad,
		Symbols: []string{mappedKind.KindName},
	})
}

func isDirErr(err error) bool {
	var pe *os.PathError
	return errors.As(err, &pe) && pe.Err == syscall.EISDIR
}
