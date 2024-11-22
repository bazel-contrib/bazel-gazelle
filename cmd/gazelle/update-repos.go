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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/internal/wspace"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

type updateReposConfig struct {
	repoFilePath  string
	importPaths   []string
	macroFileName string
	macroDefName  string
	pruneRules    bool
	workspace     *rule.File
	repoFileMap   map[string]*rule.File
}

const updateReposName = "_update-repos"

func getUpdateReposConfig(c *config.Config) *updateReposConfig {
	return c.Exts[updateReposName].(*updateReposConfig)
}

var _ config.Configurer = (*updateReposConfigurer)(nil)

type updateReposConfigurer struct{}

type macroFlag struct {
	macroFileName *string
	macroDefName  *string
}

func (f macroFlag) Set(value string) error {
	args := strings.Split(value, "%")
	if len(args) != 2 {
		return fmt.Errorf("Failure parsing to_macro: %s, expected format is macroFile%%defName", value)
	}
	if strings.HasPrefix(args[0], "..") {
		return fmt.Errorf("Failure parsing to_macro: %s, macro file path %s should not start with \"..\"", value, args[0])
	}
	*f.macroFileName = args[0]
	*f.macroDefName = args[1]
	return nil
}

func (f macroFlag) String() string {
	return ""
}

func (*updateReposConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	uc := &updateReposConfig{}
	c.Exts[updateReposName] = uc
	fs.StringVar(&uc.repoFilePath, "from_file", "", "Gazelle will translate repositories listed in this file into repository rules in WORKSPACE or a .bzl macro function. Gopkg.lock and go.mod files are supported")
	fs.Var(macroFlag{macroFileName: &uc.macroFileName, macroDefName: &uc.macroDefName}, "to_macro", "Tells Gazelle to write repository rules into a .bzl macro function rather than the WORKSPACE file. . The expected format is: macroFile%defName")
	fs.BoolVar(&uc.pruneRules, "prune", false, "When enabled, Gazelle will remove rules that no longer have equivalent repos in the go.mod file. Can only used with -from_file.")
}

func (*updateReposConfigurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	uc := getUpdateReposConfig(c)
	switch {
	case uc.repoFilePath != "":
		if len(fs.Args()) != 0 {
			return fmt.Errorf("got %d positional arguments with -from_file; wanted 0.\nTry -help for more information.", len(fs.Args()))
		}
		if !filepath.IsAbs(uc.repoFilePath) {
			uc.repoFilePath = filepath.Join(c.WorkDir, uc.repoFilePath)
		}

	default:
		if len(fs.Args()) == 0 {
			return fmt.Errorf("no repositories specified\nTry -help for more information.")
		}
		if uc.pruneRules {
			return fmt.Errorf("the -prune option can only be used with -from_file")
		}
		uc.importPaths = fs.Args()
	}

	var err error
	workspacePath := wspace.FindWORKSPACEFile(c.RepoRoot)
	uc.workspace, err = rule.LoadWorkspaceFile(workspacePath, "")
	if err != nil {
		if c.Bzlmod {
			return nil
		} else {
			return fmt.Errorf("loading WORKSPACE file: %v", err)
		}
	}
	c.Repos, uc.repoFileMap, err = repo.ListRepositories(uc.workspace)
	if err != nil {
		return fmt.Errorf("loading WORKSPACE file: %v", err)
	}

	return nil
}

func (*updateReposConfigurer) KnownDirectives() []string { return nil }

func (*updateReposConfigurer) Configure(c *config.Config, rel string, f *rule.File) {}

func updateRepos(wd string, args []string) (err error) {
	// Build configuration with all languages.
	cexts := make([]config.Configurer, 0, len(languages)+2)
	cexts = append(cexts, &config.CommonConfigurer{}, &updateReposConfigurer{})

	for _, lang := range languages {
		cexts = append(cexts, lang)
	}

	c, err := newUpdateReposConfiguration(wd, args, cexts)
	if err != nil {
		return err
	}
	uc := getUpdateReposConfig(c)

	kinds := make(map[string]rule.KindInfo)
	loads := []rule.LoadInfo{}
	for _, lang := range languages {
		if moduleAwareLang, ok := lang.(language.ModuleAwareLanguage); ok {
			loads = append(loads, moduleAwareLang.ApparentLoads(c.ModuleToApparentName)...)
		} else {
			loads = append(loads, lang.Loads()...)
		}
		for kind, info := range lang.Kinds() {
			kinds[kind] = info
		}
	}

	// TODO(jayconrod): move Go-specific RemoteCache logic to language/go.
	var knownRepos []repo.Repo

	reposFromDirectives := make(map[string]bool)
	for _, r := range c.Repos {
		if repo.IsFromDirective(r) {
			reposFromDirectives[r.Name()] = true
		}

		if r.Kind() == "go_repository" {
			knownRepos = append(knownRepos, repo.Repo{
				Name:     r.Name(),
				GoPrefix: r.AttrString("importpath"),
				Remote:   r.AttrString("remote"),
				VCS:      r.AttrString("vcs"),
			})
		}
	}
	rc, cleanup := repo.NewRemoteCache(knownRepos)
	defer func() {
		if cerr := cleanup(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	// Fix the workspace file with each language.
	for _, lang := range filterLanguages(c, languages) {
		lang.Fix(c, uc.workspace)
	}

	// Generate rules from command language arguments or by importing a file.
	var gen, empty []*rule.Rule
	if uc.repoFilePath == "" {
		gen, err = updateRepoImports(c, rc)
	} else {
		gen, empty, err = importRepos(c, rc)
	}
	if err != nil {
		return err
	}

	// Organize generated and empty rules by file. A rule should go into the file
	// it came from (by name). New rules should go into WORKSPACE or the file
	// specified with -to_macro.
	var newGen []*rule.Rule
	genForFiles := make(map[*rule.File][]*rule.Rule)
	emptyForFiles := make(map[*rule.File][]*rule.Rule)
	genNames := make(map[string]*rule.Rule)
	for _, r := range gen {

		// Skip generation of rules that are defined as directives.
		if reposFromDirectives[r.Name()] {
			continue
		}

		if existingRule := genNames[r.Name()]; existingRule != nil {
			import1 := existingRule.AttrString("importpath")
			import2 := r.AttrString("importpath")
			return fmt.Errorf("imports %s and %s resolve to the same repository rule name %s",
				import1, import2, r.Name())
		} else {
			genNames[r.Name()] = r
		}
		f := uc.repoFileMap[r.Name()]
		if f != nil {
			genForFiles[f] = append(genForFiles[f], r)
		} else {
			newGen = append(newGen, r)
		}
	}
	for _, r := range empty {
		f := uc.repoFileMap[r.Name()]
		if f == nil {
			panic(fmt.Sprintf("empty rule %q for deletion that was not found", r.Name()))
		}
		emptyForFiles[f] = append(emptyForFiles[f], r)
	}

	var macroPath string
	if uc.macroFileName != "" {
		macroPath = filepath.Join(c.RepoRoot, filepath.Clean(uc.macroFileName))
	}
	// If we are in bzlmod mode, then do not update the workspace. However, if a macro file was
	// specified, proceed with generating the macro file. This is useful for rule repositories that
	// build with bzlmod enabled, but support clients that use legacy WORKSPACE dependency loading.
	if !c.Bzlmod || macroPath != "" {
		var newGenFile *rule.File
		for f := range genForFiles {
			if macroPath == "" && wspace.IsWORKSPACE(f.Path) ||
				macroPath != "" && f.Path == macroPath && f.DefName == uc.macroDefName {
				newGenFile = f
				break
			}
		}
		if newGenFile == nil {
			if uc.macroFileName == "" {
				newGenFile = uc.workspace
			} else {
				var err error
				newGenFile, err = rule.LoadMacroFile(macroPath, "", uc.macroDefName)
				if os.IsNotExist(err) {
					newGenFile, err = rule.EmptyMacroFile(macroPath, "", uc.macroDefName)
					if err != nil {
						return fmt.Errorf("error creating %q: %v", macroPath, err)
					}
				} else if err != nil {
					return fmt.Errorf("error loading %q: %v", macroPath, err)
				}
			}
		}
		genForFiles[newGenFile] = append(genForFiles[newGenFile], newGen...)
	}

	workspaceInsertIndex := findWorkspaceInsertIndex(uc.workspace, kinds, loads)
	for _, r := range genForFiles[uc.workspace] {
		r.SetPrivateAttr(merger.UnstableInsertIndexKey, workspaceInsertIndex)
	}

	// Merge rules and fix loads in each file.
	seenFile := make(map[*rule.File]bool)
	sortedFiles := make([]*rule.File, 0, len(genForFiles))
	for f := range genForFiles {
		if !seenFile[f] {
			seenFile[f] = true
			sortedFiles = append(sortedFiles, f)
		}
	}
	for f := range emptyForFiles {
		if !seenFile[f] {
			seenFile[f] = true
			sortedFiles = append(sortedFiles, f)
		}
	}
	// If we are in bzlmod mode, then do not update the workspace.
	if !c.Bzlmod && ensureMacroInWorkspace(uc, workspaceInsertIndex) {
		if !seenFile[uc.workspace] {
			seenFile[uc.workspace] = true
			sortedFiles = append(sortedFiles, uc.workspace)
		}
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		if cmp := strings.Compare(sortedFiles[i].Path, sortedFiles[j].Path); cmp != 0 {
			return cmp < 0
		}
		return sortedFiles[i].DefName < sortedFiles[j].DefName
	})

	updatedFiles := make(map[string]*rule.File)
	for _, f := range sortedFiles {
		// We don't need to pass any wrapper macros config into MergeFile because the update repos command does not support
		// the '# gazelle:macro MACRO wraps KIND' directive.
		emptyWrapperMacros := map[string]string{}
		merger.MergeFile(f, emptyForFiles[f], genForFiles[f], merger.PreResolve, kinds, emptyWrapperMacros)
		merger.FixLoads(f, loads)
		if f == uc.workspace && !c.Bzlmod {
			if err := merger.CheckGazelleLoaded(f); err != nil {
				return err
			}
		}
		f.Sync()
		if uf, ok := updatedFiles[f.Path]; ok {
			uf.SyncMacroFile(f)
		} else {
			updatedFiles[f.Path] = f
		}
	}

	// Write updated files to disk.
	for _, f := range sortedFiles {
		if uf := updatedFiles[f.Path]; uf != nil {
			if f.DefName != "" {
				uf.SortMacro()
			}
			newContent := f.Format()
			if !bytes.Equal(f.Content, newContent) {
				if err := uf.Save(uf.Path); err != nil {
					return err
				}
			}
			delete(updatedFiles, f.Path)
		}
	}

	return nil
}

func newUpdateReposConfiguration(wd string, args []string, cexts []config.Configurer) (*config.Config, error) {
	c := config.New()
	c.WorkDir = wd
	fs := flag.NewFlagSet("gazelle", flag.ContinueOnError)
	// Flag will call this on any parse error. Don't print usage unless
	// -h or -help were passed explicitly.
	fs.Usage = func() {}
	for _, cext := range cexts {
		cext.RegisterFlags(fs, "update-repos", c)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			updateReposUsage(fs)
			return nil, err
		}
		// flag already prints the error; don't print it again.
		return nil, errors.New("Try -help for more information")
	}
	for _, cext := range cexts {
		if err := cext.CheckFlags(fs, c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func updateReposUsage(fs *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `usage:

# Add/update repositories by import path
gazelle update-repos example.com/repo1 example.com/repo2

# Import repositories from lock file
gazelle update-repos -from_file=file

The update-repos command updates repository rules in the WORKSPACE file.
update-repos can add or update repositories explicitly by import path.
update-repos can also import repository rules from a vendoring tool's lock
file (currently only deps' Gopkg.lock is supported).

FLAGS:

`)
	fs.PrintDefaults()
}

func updateRepoImports(c *config.Config, rc *repo.RemoteCache) (gen []*rule.Rule, err error) {
	// TODO(jayconrod): let the user pick the language with a command line flag.
	// For now, only use the first language that implements the interface.
	uc := getUpdateReposConfig(c)
	var updater language.RepoUpdater
	for _, lang := range filterLanguages(c, languages) {
		if u, ok := lang.(language.RepoUpdater); ok {
			updater = u
			break
		}
	}
	if updater == nil {
		return nil, fmt.Errorf("no languages can update repositories")
	}
	res := updater.UpdateRepos(language.UpdateReposArgs{
		Config:  c,
		Imports: uc.importPaths,
		Cache:   rc,
	})
	return res.Gen, res.Error
}

func importRepos(c *config.Config, rc *repo.RemoteCache) (gen, empty []*rule.Rule, err error) {
	uc := getUpdateReposConfig(c)
	importSupported := false
	var importer language.RepoImporter
	for _, lang := range filterLanguages(c, languages) {
		if i, ok := lang.(language.RepoImporter); ok {
			importSupported = true
			if i.CanImport(uc.repoFilePath) {
				importer = i
				break
			}
		}
	}
	if importer == nil {
		if importSupported {
			return nil, nil, fmt.Errorf("unknown file format: %s", uc.repoFilePath)
		} else {
			return nil, nil, fmt.Errorf("no supported languages can import configuration files")
		}
	}
	res := importer.ImportRepos(language.ImportReposArgs{
		Config: c,
		Path:   uc.repoFilePath,
		Prune:  uc.pruneRules,
		Cache:  rc,
	})
	return res.Gen, res.Empty, res.Error
}

// findWorkspaceInsertIndex reads a WORKSPACE file and finds an index within
// f.File.Stmt where new direct dependencies should be inserted. In general, new
// dependencies should be inserted after repository rules are loaded (described
// by kinds) but before macros declaring indirect dependencies.
func findWorkspaceInsertIndex(f *rule.File, kinds map[string]rule.KindInfo, loads []rule.LoadInfo) int {
	loadFiles := make(map[string]struct{})
	loadRepos := make(map[string]struct{})
	for _, li := range loads {
		name, err := label.Parse(li.Name)
		if err != nil {
			continue
		}
		loadFiles[li.Name] = struct{}{}
		loadRepos[name.Repo] = struct{}{}
	}

	// Find the first index after load statements from files that contain
	// repository rules (for example, "@bazel_gazelle//:deps.bzl") and after
	// repository rules declaring those files (http_archive for bazel_gazelle).
	// It doesn't matter whether the repository rules are actually loaded.
	insertAfter := 0

	for _, ld := range f.Loads {
		if _, ok := loadFiles[ld.Name()]; !ok {
			continue
		}
		if idx := ld.Index(); idx > insertAfter {
			insertAfter = idx
		}
	}

	for _, r := range f.Rules {
		if _, ok := loadRepos[r.Name()]; !ok {
			continue
		}
		if idx := r.Index(); idx > insertAfter {
			insertAfter = idx
		}
	}

	// There may be many direct dependencies after that index (perhaps
	// 'update-repos' inserted them previously). We want to insert after those.
	// So find the highest index after insertAfter before a call to something
	// that doesn't look like a direct dependency.
	insertBefore := len(f.File.Stmt)
	for _, r := range f.Rules {
		kind := r.Kind()
		if kind == "local_repository" || kind == "http_archive" || kind == "git_repository" {
			// Built-in or well-known repository rules.
			continue
		}
		if _, ok := kinds[kind]; ok {
			// Repository rule Gazelle might generate.
			continue
		}
		if r.Name() != "" {
			// Has a name attribute, probably still a repository rule.
			continue
		}
		if idx := r.Index(); insertAfter < idx && idx < insertBefore {
			insertBefore = idx
		}
	}

	return insertBefore
}

// ensureMacroInWorkspace adds a call to the repository macro if the -to_macro
// flag was used, and the macro was not called or declared with a
// '# gazelle:repository_macro' directive.
//
// ensureMacroInWorkspace returns true if the WORKSPACE file was updated
// and should be saved.
func ensureMacroInWorkspace(uc *updateReposConfig, insertIndex int) (updated bool) {
	if uc.macroFileName == "" {
		return false
	}

	// Check whether the macro is already declared.
	// We won't add a call if the macro is declared but not called. It might
	// be called somewhere else.
	macroValue := uc.macroFileName + "%" + uc.macroDefName
	for _, d := range uc.workspace.Directives {
		if d.Key == "repository_macro" {
			if parsed, _ := repo.ParseRepositoryMacroDirective(d.Value); parsed != nil && parsed.Path == uc.macroFileName && parsed.DefName == uc.macroDefName {
				return false
			}
		}
	}

	// Try to find a load and a call.
	var load *rule.Load
	var call *rule.Rule
	var loadedDefName string
	for _, l := range uc.workspace.Loads {
		switch l.Name() {
		case ":" + uc.macroFileName, "//:" + uc.macroFileName, "@//:" + uc.macroFileName:
			load = l
			pairs := l.SymbolPairs()
			for _, pair := range pairs {
				if pair.From == uc.macroDefName {
					loadedDefName = pair.To
				}
			}
		}
	}

	for _, r := range uc.workspace.Rules {
		if r.Kind() == loadedDefName {
			call = r
		}
	}

	// Add the load and call if they're missing.
	if call == nil {
		if load == nil {
			load = rule.NewLoad("//:" + uc.macroFileName)
			load.Insert(uc.workspace, insertIndex)
		}
		if loadedDefName == "" {
			load.Add(uc.macroDefName)
		}

		call = rule.NewRule(uc.macroDefName, "")
		call.InsertAt(uc.workspace, insertIndex)
	}

	// Add the directive to the call.
	call.AddComment("# gazelle:repository_macro " + macroValue)

	return true
}
