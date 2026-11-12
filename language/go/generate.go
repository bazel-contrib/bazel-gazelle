/* Copyright 2018 The Bazel Authors. All rights reserved.

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

package golang

import (
	"context"
	"fmt"
	"go/build"
	"log"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/pathtools"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	bzl "github.com/bazelbuild/buildtools/build"
)

func (gl *goLang) Generate(ctx context.Context, args language.GenerateArgs) (language.GenerateResult, error) {
	// Extract information about proto files. We need this to exclude .pb.go
	// files and generate go_proto_library rules.
	c := args.Config
	gc := getGoConfig(c)
	pcMode := getProtoMode(c)

	// This is a collection of proto_library rule names that have a corresponding
	// go_proto_library rule already generated.
	goProtoRules := make(map[string]struct{})

	var protoRuleNames []string
	protoPackages := make(map[string]proto.Package)
	protoFileInfo := make(map[string]proto.FileInfo)
	for _, r := range args.OtherGen {
		if r.Kind() == "go_proto_library" {
			if proto := r.AttrString("proto"); proto != "" {
				goProtoRules[proto] = struct{}{}
			}
			if protos := r.AttrStrings("protos"); protos != nil {
				for _, proto := range protos {
					goProtoRules[proto] = struct{}{}
				}
			}

		}
		if r.Kind() != "proto_library" {
			continue
		}
		pkg := r.PrivateAttr(proto.PackageKey).(proto.Package)
		protoPackages[r.Name()] = pkg
		for name, info := range pkg.Files {
			protoFileInfo[name] = info
		}
		protoRuleNames = append(protoRuleNames, r.Name())
	}
	sort.Strings(protoRuleNames)
	var emptyProtoRuleNames []string
	for _, r := range args.OtherEmpty {
		if r.Kind() == "proto_library" {
			emptyProtoRuleNames = append(emptyProtoRuleNames, r.Name())
		}
	}

	// If proto rule generation is enabled, exclude .pb.go files that correspond
	// to any .proto files present.
	regularFiles := append([]string{}, args.RegularFiles...)
	genFiles := append([]string{}, args.GenFiles...)
	if !pcMode.ShouldIncludePregeneratedFiles() {
		keep := func(f string) bool {
			for _, suffix := range []string{".pb.go", "_grpc.pb.go"} {
				if strings.HasSuffix(f, suffix) {
					if _, ok := protoFileInfo[strings.TrimSuffix(f, suffix)+".proto"]; ok {
						return false
					}
				}
			}
			return true
		}
		filterFiles(&regularFiles, keep)
		filterFiles(&genFiles, keep)
	}

	// Split regular files into files which can determine the package name and
	// import path and other files.
	var goFiles, otherFiles []string
	for _, f := range regularFiles {
		if strings.HasSuffix(f, ".go") {
			goFiles = append(goFiles, f)
		} else {
			otherFiles = append(otherFiles, f)
		}
	}

	// Look for a subdirectory named testdata. Only treat it as data if it does
	// not contain a buildable package and is not empty.
	var hasTestdata bool
	for _, sub := range args.Subdirs {
		if sub == "testdata" {
			_, ok := gl.goPkgRels[path.Join(args.Rel, "testdata")]

			// Check that testdata directory is not empty
			if !ok {
				testdataRel := path.Join(args.Rel, "testdata")
				testdataDir, err := args.Cache.GetDirInfo(testdataRel)
				if err == nil && (len(testdataDir.Subdirs) > 0 || len(testdataDir.RegularFiles) > 0 || len(testdataDir.GenFiles) > 0) {
					hasTestdata = true
				}
			}
			break
		}
	}

	// Build a set of packages from files in this directory.
	srcdir := args.Rel
	if gc.goRepositoryMode {
		// cgo opts such as '-L${SRCDIR}/libs' should become
		// '-Lexternal/my_repo~/libs' in an external repo.
		// We obtain the path from the repo root to support both cases of
		// --experimental_sibling_repository_layout.
		slashPath := filepath.ToSlash(c.RepoRoot)
		segments := strings.Split(slashPath, "/")
		repoName := segments[len(segments)-1]
		previousSegment := segments[len(segments)-2]
		if previousSegment == "external" {
			srcdir = path.Join("external", repoName, srcdir)
		} else {
			srcdir = path.Join("..", repoName, srcdir)
		}
	}
	goFileInfos := make([]fileInfo, len(goFiles))
	var er *embedResolver
	for i, name := range goFiles {
		path := filepath.Join(args.Dir, name)
		goFileInfos[i] = goFileInfo(path, srcdir)
		if len(goFileInfos[i].embeds) > 0 && er == nil {
			er = newEmbedResolver(args.Dir, args.Rel, c.ValidBuildFileNames, gl.goPkgRels, args.Subdirs, args.RegularFiles, args.GenFiles)
		}
	}
	goPackageMap, goFilesWithUnknownPackage := buildPackages(c, args.Dir, args.Rel, hasTestdata, er, goFileInfos)

	// Select a package to generate rules for. If there is no package, create
	// an empty package so we can generate empty rules.
	var protoName string
	pkg, err := selectPackage(c, args.Dir, goPackageMap)
	if err != nil {
		if _, ok := err.(*build.NoGoError); ok {
			if len(protoPackages) == 1 {
				for name, ppkg := range protoPackages {
					if _, ok := goProtoRules[":"+name]; ok {
						// if a go_proto_library rule already exists for this
						// proto package, treat it as if the proto package
						// doesn't exist.
						pkg = emptyPackage(c, args.Dir, args.Rel, args.File)
						break
					}
					pkg = &goPackage{
						name:       goProtoPackageName(ppkg),
						importPath: goProtoImportPath(c, ppkg, args.Rel),
						proto:      protoTargetFromProtoPackage(name, ppkg),
					}
					protoName = name
					break
				}
			} else {
				pkg = emptyPackage(c, args.Dir, args.Rel, args.File)
			}
		} else {
			log.Print(err)
		}
	}

	// Try to link the selected package with a proto package.
	if pkg != nil {
		if pkg.importPath == "" {
			if err := pkg.inferImportPath(c); err != nil && pkg.firstGoFile() != "" {
				inferImportPathErrorOnce.Do(func() { log.Print(err) })
			}
		}
		for _, name := range protoRuleNames {
			ppkg := protoPackages[name]
			if pkg.importPath == goProtoImportPath(c, ppkg, args.Rel) {
				protoName = name
				pkg.proto = protoTargetFromProtoPackage(name, ppkg)
				break
			}
		}
	}

	// Generate rules for proto packages. These should come before the other
	// Go rules.
	g := newGenerator(c, gc, args)
	for _, info := range goFileInfos {
		if info.isCgo {
			g.cgoGoFiles[info.name] = true
		}
	}
	var res language.GenerateResult
	var rules []*rule.Rule
	var protoEmbeds []string
	var protoEmbed string
	if pcMode == proto.FileMode {
		// get the list of protoRuleNames that are not already in goProtoRules
		var newProtoRuleNames []string
		for _, name := range protoRuleNames {
			if _, ok := goProtoRules[":"+name]; !ok {
				newProtoRuleNames = append(newProtoRuleNames, name)
			}
		}

		// get the protoTargets for the new protoRuleNames
		var importPathToProtoTargets = make(map[string][]protoTarget)
		for _, name := range newProtoRuleNames {
			ppkg := protoPackages[name]
			importPath := goProtoImportPath(c, ppkg, args.Rel)
			importPathToProtoTargets[importPath] = append(importPathToProtoTargets[importPath], protoTargetFromProtoPackage(name, ppkg))
		}

		// Deterministically sort by order of importpath.
		importPaths := []string{}
		for importPath, _ := range importPathToProtoTargets {
			importPaths = append(importPaths, importPath)
		}
		sort.Strings(importPaths)

		for _, importPath := range importPaths {
			var rs []*rule.Rule
			protoTargets := importPathToProtoTargets[importPath]
			protoEmbed, rs = g.generateProto(pcMode, protoTargets, importPath)
			if protoEmbed != "" {
				// check if rs is non-empty and that the first rule is a go_proto_library with the same importPath
				if len(rs) > 0 && rs[0].Kind() == "go_proto_library" && rs[0].AttrString("importpath") == pkg.importPath {
					protoEmbeds = append(protoEmbeds, protoEmbed)
				}
			}
			rules = append(rules, rs...)
		}
	} else {
		for _, name := range protoRuleNames {
			// if a go_proto_library rule exists for this proto_library rule
			// already, skip creating another go_proto_library for it, assuming
			// that a different gazelle extension is responsible for
			// go_proto_library rule generation.
			if _, ok := goProtoRules[":"+name]; ok {
				continue
			}
			ppkg := protoPackages[name]
			var rs []*rule.Rule
			if name == protoName {
				var protoTargets []protoTarget
				if pkg != nil {
					protoTargets = append(protoTargets, pkg.proto)
				}
				protoEmbed, rs = g.generateProto(pcMode, []protoTarget{pkg.proto}, pkg.importPath)
				if protoEmbed != "" {
					// check if rs is non-empty and that the first rule is a go_proto_library with the same importPath
					if len(rs) > 0 && rs[0].Kind() == "go_proto_library" && rs[0].AttrString("importpath") == pkg.importPath {
						protoEmbeds = append(protoEmbeds, protoEmbed)
					}
				}
			} else {
				target := protoTargetFromProtoPackage(name, ppkg)
				importPath := goProtoImportPath(c, ppkg, args.Rel)
				_, rs = g.generateProto(pcMode, []protoTarget{target}, importPath)
			}
			rules = append(rules, rs...)
		}
	}

	for _, name := range emptyProtoRuleNames {
		goProtoName := strings.TrimSuffix(name, "_proto") + goProtoSuffix
		res.Empty = append(res.Empty, rule.NewRule("go_proto_library", goProtoName))
	}
	if pkg != nil && (pcMode == proto.PackageMode || pcMode == proto.FileMode) && pkg.firstGoFile() == "" {
		// In proto package mode, don't generate a go_library embedding a
		// go_proto_library unless there are actually go files.
		protoEmbeds = nil
	}

	// Complete the Go package and generate rules for that.
	if pkg != nil {
		// Add files with unknown packages. This happens when there are parse
		// or I/O errors. We should keep the file in the srcs list and let the
		// compiler deal with the error.
		cgo := pkg.haveCgo()
		for _, info := range goFilesWithUnknownPackage {
			if err := pkg.addFile(c, er, info, cgo); err != nil {
				log.Print(err)
			}
		}

		// Process the other static files.
		for _, file := range otherFiles {
			info := otherFileInfo(filepath.Join(args.Dir, file))
			if err := pkg.addFile(c, er, info, cgo); err != nil {
				log.Print(err)
			}
		}

		// Process generated files. Note that generated files may have the same names
		// as static files. Bazel will use the generated files, but we will look at
		// the content of static files, assuming they will be the same.
		regularFileSet := make(map[string]bool)
		for _, f := range regularFiles {
			regularFileSet[f] = true
		}
		// Some of the generated files may have been consumed by other rules
		consumedFileSet := make(map[string]bool)
		for _, r := range args.OtherGen {
			for _, f := range r.AttrStrings("srcs") {
				consumedFileSet[f] = true
			}
			if f := r.AttrString("src"); f != "" {
				consumedFileSet[f] = true
			}
		}
		for _, f := range genFiles {
			if regularFileSet[f] || consumedFileSet[f] {
				continue
			}
			info := fileNameInfo(filepath.Join(args.Dir, f))
			if err := pkg.addFile(c, er, info, cgo); err != nil {
				log.Print(err)
			}
		}

		var genGoProtoRules []string
		for _, r := range rules {
			if r.Kind() == "go_proto_library" {
				genGoProtoRules = append(genGoProtoRules, r.Name())
			}
		}

		// Generate Go rules.
		if protoName == "" {
			// Empty proto rules for deletion.
			_, rs := g.generateProto(pcMode, []protoTarget{pkg.proto}, pkg.importPath)

			// Do not generate empty rule if already created above
			found := false
			for _, r := range rs {
				if r.Kind() == "go_proto_library" {
					for _, pr := range genGoProtoRules {
						if pr == r.Name() {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				rules = append(rules, rs...)
			}
		}
		lib := g.generateLib(pkg, protoEmbeds)
		var libName string
		if !lib.IsEmpty(goKindsMap[lib.Kind()]) {
			libName = lib.Name()
		}
		rules = append(rules, lib)
		g.maybePublishToolLib(lib, pkg)
		if r := g.maybeGenerateExtraLib(lib, pkg); r != nil {
			rules = append(rules, r)
		}
		if r := g.maybeGenerateAlias(pkg, libName); r != nil {
			g.maybePublishToolLib(r, pkg)
			rules = append(rules, r)
		}
		rules = append(rules, g.generateBin(pkg, libName))
		rules = append(rules, g.generateTests(pkg, libName)...)
	}

	for _, r := range rules {
		if r.IsEmpty(goKindsMap[r.Kind()]) {
			res.Empty = append(res.Empty, r)
		} else {
			res.Gen = append(res.Gen, r)
			rawImports := r.PrivateAttr(gazelleImportsKey)
			res.Imports = append(res.Imports, rawImports)
			if imports, ok := rawImports.(rule.PlatformStrings); ok && g.shouldIndex {
				g.addRelsToIndex(imports)
			}
		}
	}

	for r := range g.relsToIndexSeen {
		res.RelsToIndex = append(res.RelsToIndex, r)
	}
	sort.Strings(res.RelsToIndex) // for deterministic output

	if args.File != nil || len(res.Gen) > 0 {
		gl.goPkgRels[args.Rel] = true
	} else {
		for _, sub := range args.Subdirs {
			if _, ok := gl.goPkgRels[path.Join(args.Rel, sub)]; ok {
				gl.goPkgRels[args.Rel] = false
				break
			}
		}
	}

	return res, nil
}

func filterFiles(files *[]string, pred func(string) bool) {
	w := 0
	for r := 0; r < len(*files); r++ {
		f := (*files)[r]
		if pred(f) {
			(*files)[w] = f
			w++
		}
	}
	*files = (*files)[:w]
}

func buildPackages(c *config.Config, dir, rel string, hasTestdata bool, er *embedResolver, goFiles []fileInfo) (packageMap map[string]*goPackage, goFilesWithUnknownPackage []fileInfo) {
	// Process .go and .proto files first, since these determine the package name.
	packageMap = make(map[string]*goPackage)
	for _, f := range goFiles {
		if f.packageName == "" {
			goFilesWithUnknownPackage = append(goFilesWithUnknownPackage, f)
			continue
		}
		if f.packageName == "documentation" {
			// go/build ignores this package
			continue
		}

		if _, ok := packageMap[f.packageName]; !ok {
			packageMap[f.packageName] = &goPackage{
				name:        f.packageName,
				dir:         dir,
				rel:         rel,
				hasTestdata: hasTestdata,
			}
		}
		if err := packageMap[f.packageName].addFile(c, er, f, false); err != nil {
			log.Print(err)
		}
	}
	return packageMap, goFilesWithUnknownPackage
}

var inferImportPathErrorOnce sync.Once

// selectPackages selects one Go packages out of the buildable packages found
// in a directory. If multiple packages are found, it returns the package
// whose name matches the directory if such a package exists.
func selectPackage(c *config.Config, dir string, packageMap map[string]*goPackage) (*goPackage, error) {
	buildablePackages := make(map[string]*goPackage)
	for name, pkg := range packageMap {
		if pkg.isBuildable(c) {
			buildablePackages[name] = pkg
		}
	}

	if len(buildablePackages) == 0 {
		return nil, &build.NoGoError{Dir: dir}
	}

	if len(buildablePackages) == 1 {
		for _, pkg := range buildablePackages {
			return pkg, nil
		}
	}

	if pkg, ok := buildablePackages[defaultPackageName(c, dir)]; ok {
		return pkg, nil
	}

	err := &build.MultiplePackageError{Dir: dir}
	for name, pkg := range buildablePackages {
		// Add the first file for each package for the error message.
		// Error() method expects these lists to be the same length. File
		// lists must be non-empty. These lists are only created by
		// buildPackage for packages with .go files present.
		err.Packages = append(err.Packages, name)
		err.Files = append(err.Files, pkg.firstGoFile())
	}
	return nil, err
}

func emptyPackage(c *config.Config, dir, rel string, f *rule.File) *goPackage {
	var pkgName string
	if fileContainsGoBinary(c, f) {
		// If the file contained a go_binary, its library may have a "_lib" suffix.
		// Set the package name to "main" so that we generate an empty library rule
		// with that name.
		pkgName = "main"
	} else {
		pkgName = defaultPackageName(c, dir)
	}
	pkg := &goPackage{
		name: pkgName,
		dir:  dir,
		rel:  rel,
	}

	return pkg
}

func defaultPackageName(c *config.Config, rel string) string {
	gc := getGoConfig(c)
	return pathtools.RelBaseName(rel, gc.prefix, "")
}

type generator struct {
	c                   *config.Config
	gc                  *goConfig
	rel                 string
	shouldSetVisibility bool

	shouldIndex     bool
	relsToIndexSeen map[string]struct{}

	// file is the existing build file, or nil if there is none.
	file *rule.File

	// cgoGoFiles is the set of .go files in the directory that import "C",
	// whether or not they match a platform.
	cgoGoFiles map[string]bool
}

func newGenerator(c *config.Config, gc *goConfig, args language.GenerateArgs) *generator {
	g := &generator{
		c:                   c,
		gc:                  gc,
		rel:                 args.Rel,
		shouldSetVisibility: shouldSetVisibility(args),
		shouldIndex:         c.IndexLibraries && len(gc.goSearch) > 0,
		file:                args.File,
		cgoGoFiles:          make(map[string]bool),
	}
	if g.shouldIndex {
		g.relsToIndexSeen = make(map[string]struct{})
	}
	return g
}

func (g *generator) generateProto(mode proto.Mode, targets []protoTarget, importPath string) (string, []*rule.Rule) {
	if !mode.ShouldGenerateRules() && mode != proto.LegacyMode {
		// Don't create or delete proto rules in this mode. When proto mode is disabled,
		// there may be hand-written rules or pre-generated Go files
		return "", nil
	}

	gc := getGoConfig(g.c)
	filegroupName := legacyProtoFilegroupName
	var protoName string
	if len(targets) == 1 {
		protoName = targets[0].name
	}
	if protoName == "" {
		if importPath == "" {
			importPath = InferImportPath(g.c, g.rel)
		}
		protoName = proto.RuleName(importPath)
	}
	goProtoName := strings.TrimSuffix(protoName, "_proto") + goProtoSuffix

	if mode == proto.LegacyMode {
		filegroup := rule.NewRule("filegroup", filegroupName)
		if targets[0].sources.isEmpty() {
			return "", []*rule.Rule{filegroup}
		}
		filegroup.SetAttr("srcs", targets[0].sources.build())
		if g.shouldSetVisibility {
			filegroup.SetAttr("visibility", g.commonVisibility(importPath))
		}
		return "", []*rule.Rule{filegroup}
	}

	var atLeastOneTargetHasSources bool
	for _, target := range targets {
		if !target.sources.isEmpty() {
			atLeastOneTargetHasSources = true
			break
		}
	}

	if !atLeastOneTargetHasSources {
		return "", []*rule.Rule{
			rule.NewRule("filegroup", filegroupName),
			rule.NewRule("go_proto_library", goProtoName),
		}
	}

	goProtoLibrary := rule.NewRule("go_proto_library", goProtoName)
	if len(targets) == 1 {
		goProtoLibrary.SetAttr("proto", ":"+protoName)
	} else {
		// generate protoNames with prefix ":"
		protoNames := make([]string, len(targets))
		for i := range targets {
			protoNames[i] = ":" + targets[i].name
		}
		goProtoLibrary.SetAttr("protos", protoNames)
	}

	g.setImportAttrs(goProtoLibrary, importPath)
	var atLeastOneTargetHasServices bool
	for _, target := range targets {
		if target.hasServices {
			atLeastOneTargetHasServices = true
			break
		}
	}
	if atLeastOneTargetHasServices {
		if gc.goGrpcCompilers != nil {
			goProtoLibrary.SetAttr("compilers", gc.goGrpcCompilers)
		} else {
			goProtoLibrary.SetAttr("compilers", gc.defaultGoGrpcCompilers())
		}
	} else if gc.goProtoCompilers != nil {
		goProtoLibrary.SetAttr("compilers", gc.goProtoCompilers)
	}
	if g.shouldSetVisibility {
		goProtoLibrary.SetAttr("visibility", g.commonVisibility(importPath))
	}
	if len(targets) == 1 {
		goProtoLibrary.SetPrivateAttr(gazelleImportsKey, targets[0].imports.build())
	} else {
		protoSources := make(map[string]struct{})
		for _, target := range targets {
			for _, src := range target.sources.build().Generic {
				// if src starts with the repo root plus a slash, trim it
				// so that src is relative to the repo root, which is
				// needed for the dep resolution to work correctly
				if strings.HasPrefix(src, g.c.RepoRoot+"/") {
					src = src[len(g.c.RepoRoot)+1:]
				}
				protoSources[src] = struct{}{}
			}
		}
		var combinedImports platformStringsBuilder
		for _, target := range targets {
			builtImports := target.imports.build()
			// handle Generics
			for _, genericString := range builtImports.Generic {
				// if the generic string is not in the set of generic src strings, add it
				if _, ok := protoSources[genericString]; !ok {
					combinedImports.addGenericString(genericString)
				}
			}
		}

		goProtoLibrary.SetPrivateAttr(gazelleImportsKey, combinedImports.build())
	}
	return goProtoName, []*rule.Rule{goProtoLibrary}
}

func (g *generator) generateLib(pkg *goPackage, embeds []string) *rule.Rule {
	gc := getGoConfig(g.c)
	name := libNameByConvention(gc.goNamingConvention, pkg.importPath, pkg.name)
	goLibrary := rule.NewRule("go_library", name)
	if !pkg.library.sources.hasGo() && len(embeds) == 0 {
		return goLibrary // empty
	}
	var visibility []string
	if pkg.isCommand() {
		// By default, libraries made for a go_binary should not be exposed to the public.
		visibility = []string{"//visibility:private"}
		if len(getGoConfig(g.c).goVisibility) > 0 {
			visibility = getGoConfig(g.c).goVisibility
		}
	} else if g.shouldSetVisibility {
		visibility = g.commonVisibility(pkg.importPath)
	}
	g.setCommonAttrs(goLibrary, pkg.rel, visibility, pkg.library, embeds)
	g.setImportAttrs(goLibrary, pkg.importPath)
	return goLibrary
}

func (g *generator) maybeGenerateAlias(pkg *goPackage, libName string) *rule.Rule {
	if pkg.isCommand() || libName == "" {
		return nil
	}
	gc := getGoConfig(g.c)
	if gc.goNamingConvention == goDefaultLibraryNamingConvention {
		return nil
	}
	alias := rule.NewRule("alias", defaultLibName)
	alias.SetAttr("visibility", g.commonVisibility(pkg.importPath))
	if gc.goNamingConvention == importAliasNamingConvention {
		alias.SetAttr("actual", ":"+libName)
	}
	return alias
}

func (g *generator) generateBin(pkg *goPackage, library string) *rule.Rule {
	gc := getGoConfig(g.c)
	name := binName(pkg.rel, gc.prefix, g.c.RepoRoot)
	goBinary := rule.NewRule("go_binary", name)
	if !pkg.isCommand() || pkg.binary.sources.isEmpty() && library == "" {
		return goBinary // empty
	}
	var visibility []string
	if g.shouldSetVisibility {
		visibility = g.commonVisibility(pkg.importPath)
	}
	g.setCommonAttrs(goBinary, pkg.rel, visibility, pkg.binary, []string{library})
	return goBinary
}

func (g *generator) generateTests(pkg *goPackage, library string) []*rule.Rule {
	gc := getGoConfig(g.c)
	tests := pkg.tests
	if len(tests) == 0 && gc.testMode == defaultTestMode {
		tests = []goTarget{goTarget{}}
	}
	var name func(goTarget) string
	switch gc.testMode {
	case defaultTestMode:
		name = func(goTarget) string {
			return testNameByConvention(gc.goNamingConvention, pkg.importPath)
		}
	case fileTestMode:
		name = func(test goTarget) string {
			if test.sources.hasGo() {
				if srcs := test.sources.buildFlat(); len(srcs) == 1 {
					return testNameFromSingleSource(srcs[0])
				}
			}
			return testNameByConvention(gc.goNamingConvention, pkg.importPath)
		}
	}
	var res []*rule.Rule
	for i, test := range tests {
		goTest := rule.NewRule("go_test", name(test))
		hasGo := test.sources.hasGo()
		if hasGo || i == 0 {
			res = append(res, goTest)
			if !hasGo {
				continue
			}
		}
		var embeds []string
		if test.hasInternalTest {
			if library != "" {
				embeds = append(embeds, library)
			}
		}
		g.setCommonAttrs(goTest, pkg.rel, nil, test, embeds)
		if pkg.hasTestdata {
			goTest.SetAttr("data", rule.GlobValue{Patterns: []string{"testdata/**"}})
		}
	}
	return res
}

// maybePublishToolLib makes the given go_library rule public if needed for nogo.
// Updating it here automatically makes it easier to upgrade org_golang_x_tools.
func (g *generator) maybePublishToolLib(lib *rule.Rule, pkg *goPackage) {
	if pkg.importPath == "golang.org/x/tools/go/analysis/internal/facts" || pkg.importPath == "golang.org/x/tools/internal/facts" {
		// Imported by nogo main. We add a visibility exception.
		lib.SetAttr("visibility", []string{"//visibility:public"})
	}
}

// maybeGenerateExtraLib generates extra equivalent library targets for
// certain protobuf libraries. Historically, these "_gen" targets depend on Well Known Types
// built with go_proto_library and are used together with go_proto_library.
// However, these are no longer needed and are kept as aliases to be backward-compatible
func (g *generator) maybeGenerateExtraLib(lib *rule.Rule, pkg *goPackage) *rule.Rule {
	gc := getGoConfig(g.c)
	if gc.prefix != "github.com/golang/protobuf" || gc.prefixRel != "" {
		return nil
	}

	var r *rule.Rule
	switch pkg.importPath {
	case "github.com/golang/protobuf/descriptor":
		r = rule.NewRule("alias", "go_default_library_gen")
		r.SetAttr("actual", ":go_default_library")
		r.SetAttr("visibility", []string{"//visibility:public"})

	case "github.com/golang/protobuf/jsonpb":
		r = rule.NewRule("alias", "go_default_library_gen")
		r.SetAttr("actual", ":go_default_library")
		r.SetAttr("visibility", []string{"//visibility:public"})

	case "github.com/golang/protobuf/protoc-gen-go/generator":
		r = rule.NewRule("alias", "go_default_library_gen")
		r.SetAttr("actual", ":go_default_library")
		r.SetAttr("visibility", []string{"//visibility:public"})

	case "github.com/golang/protobuf/ptypes":
		r = rule.NewRule("alias", "go_default_library_gen")
		r.SetAttr("actual", ":go_default_library")
		r.SetAttr("visibility", []string{"//visibility:public"})
	}

	return r
}

func (g *generator) setCommonAttrs(r *rule.Rule, pkgRel string, visibility []string, target goTarget, embeds []string) {
	gc := getGoConfig(g.c)
	linksBinary := r.Kind() == "go_binary" || r.Kind() == "go_test"
	constraintPrefix := "@" + gc.rulesGoRepoName + "//go/platform:"

	// rules_go fails analysis when cgo is False and the rule has cdeps, C
	// compiler or linker flags, or C and C++ sources. Gazelle does not
	// generate cdeps, and merge keeps some inputs of the existing rules. Turn
	// cgo on wherever such inputs remain. Only targets with a .go file that
	// imports "C" get here. Earlier versions of Gazelle set cgo = True on all
	// platforms for these targets, so this never turns on cgo where earlier
	// versions left it off.
	if si, ok := target.cgoPlatforms.cgoConstraintInfo(); target.cgo && (!ok || si.set != genericSet) {
		g.addKeptCInputPlatforms(&target.cgoPlatforms, r.Name())
		target.cgoPlatforms.collapseAllSupportedPlatforms(gc.rulesGoVersion, constraintPrefix)
	}

	// Merge directive-provided C compiler/linker flags into the flags Gazelle
	// derived from #cgo comments. These attributes only apply to cgo targets,
	// which in practice means go_library: cgo is not allowed in _test.go files,
	// and a cgo main is generated as a cgo go_library plus a go_binary that
	// embeds it. The flags apply on the platforms where cgo is on. Adding them
	// before build() lets the emission below deduplicate and platform-merge
	// them with the flags from #cgo comments.
	if cgoSI, ok := target.cgoPlatforms.cgoConstraintInfo(); ok {
		for _, opt := range gc.copts {
			target.copts.addConstrainedString(opt, cgoSI, constraintPrefix)
		}
		for _, opt := range gc.cppopts {
			target.cppopts.addConstrainedString(opt, cgoSI, constraintPrefix)
		}
		for _, opt := range gc.cxxopts {
			target.cxxopts.addConstrainedString(opt, cgoSI, constraintPrefix)
		}
		for _, opt := range gc.clinkopts {
			target.clinkopts.addConstrainedString(opt, cgoSI, constraintPrefix)
		}
	}

	if !target.sources.isEmpty() {
		r.SetAttr("srcs", target.buildSources(constraintPrefix))
	}
	if !target.embedSrcs.isEmpty() {
		r.SetAttr("embedsrcs", target.embedSrcs.build())
	}
	if cgoAttr := target.cgoPlatforms.buildCgoAttr(gc.rulesGoVersion, constraintPrefix); cgoAttr != nil {
		r.SetAttr("cgo", cgoAttr)
	}
	if target.pgoprofile != "" {
		r.SetAttr("pgoprofile", target.pgoprofile)
	}
	if !target.clinkopts.isEmpty() {
		r.SetAttr("clinkopts", g.options(target.clinkopts.build(), pkgRel))
	}
	if !target.cppopts.isEmpty() {
		r.SetAttr("cppopts", g.options(target.cppopts.build(), pkgRel))
	}
	if !target.copts.isEmpty() {
		r.SetAttr("copts", g.options(target.copts.build(), pkgRel))
	}
	if !target.cxxopts.isEmpty() {
		r.SetAttr("cxxopts", g.options(target.cxxopts.build(), pkgRel))
	}
	// Go compiler / linker flags. Unlike the C flags above these are not derived
	// from source, so they are set directly from the directive. gc_goopts affects
	// compilation, so it is only set on rules that carry sources (e.g. not on a
	// go_binary that merely embeds a library). gc_linkopts affects linking, so it
	// is only set on rules that link a binary (go_binary, go_test).
	if len(gc.gcGoopts) > 0 && !target.sources.isEmpty() {
		r.SetAttr("gc_goopts", gc.gcGoopts)
	}
	if linksBinary && len(gc.gcLinkopts) > 0 {
		r.SetAttr("gc_linkopts", gc.gcLinkopts)
	}
	if g.shouldSetVisibility && len(visibility) > 0 {
		r.SetAttr("visibility", visibility)
	}
	if len(embeds) > 0 {
		colonEmbeds := make([]string, 0, len(embeds))
		for _, embed := range embeds {
			colonEmbeds = append(colonEmbeds, ":"+embed)
		}
		r.SetAttr("embed", colonEmbeds)
	}
	r.SetPrivateAttr(gazelleImportsKey, target.imports.build())
}

// cInputAttrs are the attributes that rules_go rejects when cgo is False.
// For srcs, only C and C++ files count.
var cInputAttrs = []string{"cdeps", "clinkopts", "copts", "cppopts", "cxxopts", "srcs"}

// addKeptCInputPlatforms adds to cgoPlatforms the platforms where C inputs
// of existing rules remain after the generated rule named name is merged.
//
// Two kinds of existing rules count. The first is the rule named name, unless
// it has a keep comment, because merge does not change such a rule. The
// second is a rule that embeds the target and does not set cgo itself:
// rules_go uses the cgo setting of the embedded target for that rule, so all
// of its C inputs count.
func (g *generator) addKeptCInputPlatforms(cgoPlatforms *platformStringsBuilder, name string) {
	if g.file == nil {
		return
	}
	for _, r := range g.file.Rules {
		switch {
		case r.Name() == name:
			if !r.ShouldKeep() {
				g.addCInputPlatforms(cgoPlatforms, r, false)
			}
		case strListAttrContains(r, "embed", ":"+name) && !setsCgo(r):
			g.addCInputPlatforms(cgoPlatforms, r, true)
		}
	}
}

// addCInputPlatforms adds to cgoPlatforms the platforms of the C inputs of r
// that remain after a merge. If all is false, only inputs that merge keeps
// count. cdeps is not mergeable, so merge keeps all of it. Merge also keeps
// all of an attribute that has a keep comment or that it cannot parse.
// FlattenExpr returns such an expression unchanged. In other attributes,
// merge keeps only the values that have a keep comment.
func (g *generator) addCInputPlatforms(cgoPlatforms *platformStringsBuilder, r *rule.Rule, all bool) {
	for _, key := range cInputAttrs {
		value := r.Attr(key)
		if value == nil {
			continue
		}
		keepAll := all || key == "cdeps" || shouldKeepAttr(r, key) || rule.FlattenExpr(value) == value
		walkSelectValues(value, "", func(v bzl.Expr, selectKey string) {
			if !keepAll && !rule.ShouldKeep(v) {
				return
			}
			if key == "srcs" && !g.isCSrc(v) {
				return
			}
			g.addCgoPlatformsForSelectKey(cgoPlatforms, selectKey)
		})
	}
}

// walkSelectValues calls f for each value in e, which may be a list, a
// select, or a concatenation of these. selectKey is the select key that
// applies to the value, or "" outside a select. f gets any other expression
// as one value.
func walkSelectValues(e bzl.Expr, selectKey string, f func(v bzl.Expr, selectKey string)) {
	switch e := e.(type) {
	case *bzl.ListExpr:
		for _, v := range e.List {
			f(v, selectKey)
		}
		return
	case *bzl.BinaryExpr:
		if e.Op == "+" {
			walkSelectValues(e.X, selectKey, f)
			walkSelectValues(e.Y, selectKey, f)
			return
		}
	case *bzl.CallExpr:
		if x, ok := e.X.(*bzl.Ident); ok && x.Name == "select" && len(e.List) == 1 {
			if dict, ok := e.List[0].(*bzl.DictExpr); ok {
				for _, kv := range dict.List {
					key := ""
					if s, ok := kv.Key.(*bzl.StringExpr); ok {
						key = s.Value
					}
					walkSelectValues(kv.Value, key, f)
				}
				return
			}
		}
	}
	f(e, selectKey)
}

// isCSrc reports whether a value in srcs names a file that rules_go builds
// only with cgo: a companion file, or a .go file that imports "C". Gazelle
// cannot see the files of a value that is not a string, so such a value
// counts too.
func (g *generator) isCSrc(v bzl.Expr) bool {
	s, ok := v.(*bzl.StringExpr)
	if !ok {
		return true
	}
	if s.Value == "" {
		return false
	}
	return g.cgoGoFiles[s.Value] || isCompanionCgoFile(fileNameInfo(s.Value))
}

// addCgoPlatformsForSelectKey adds to cgoPlatforms the platform that a select
// key matches. The key must be the rules_go config_setting of a known OS,
// architecture, or OS and architecture. Any other key, including
// "//conditions:default" and "" for values outside a select, counts as all
// platforms.
func (g *generator) addCgoPlatformsForSelectKey(cgoPlatforms *platformStringsBuilder, selectKey string) {
	constraintPrefix := "@" + g.gc.rulesGoRepoName + "//go/platform:"
	if name, ok := strings.CutPrefix(selectKey, constraintPrefix); ok {
		if rule.KnownOSSet[name] {
			cgoPlatforms.addOSString(cgoSentinel, []string{name}, constraintPrefix)
			return
		}
		if rule.KnownArchSet[name] {
			cgoPlatforms.addArchString(cgoSentinel, []string{name}, constraintPrefix)
			return
		}
		for _, p := range rule.KnownPlatforms {
			if p.String() == name {
				cgoPlatforms.addPlatformString(cgoSentinel, []rule.Platform{p}, constraintPrefix)
				return
			}
		}
	}
	cgoPlatforms.addGenericString(cgoSentinel)
}

// setsCgo reports whether r sets cgo to a value other than False.
func setsCgo(r *rule.Rule) bool {
	cgo := r.Attr("cgo")
	if cgo == nil {
		return false
	}
	ident, ok := cgo.(*bzl.Ident)
	return !ok || ident.Name != "False"
}

func (g *generator) setImportAttrs(r *rule.Rule, importPath string) {
	gc := getGoConfig(g.c)
	r.SetAttr("importpath", importPath)

	// Set importpath_aliases if we need minimal module compatibility.
	// If a package is part of a module with a v2+ semantic import version
	// suffix, packages that are not part of modules may import it without
	// the suffix.
	if gc.goRepositoryMode && gc.moduleMode && pathtools.HasPrefix(importPath, gc.prefix) && gc.prefixRel == "" {
		if mmcImportPath := pathWithoutSemver(importPath); mmcImportPath != "" {
			r.SetAttr("importpath_aliases", []string{mmcImportPath})
		}
	}

	if gc.importMapPrefix != "" {
		fromPrefixRel := pathtools.TrimPrefix(g.rel, gc.importMapPrefixRel)
		importMap := path.Join(gc.importMapPrefix, fromPrefixRel)
		if importMap != importPath {
			r.SetAttr("importmap", importMap)
		}
	}
}

func (g *generator) commonVisibility(importPath string) []string {
	// If the Bazel package name (rel) contains "internal", add visibility for
	// subpackages of the parent.
	// If the import path contains "internal" but rel does not, this is
	// probably an internal submodule. Add visibility for all subpackages.
	relIndex := pathtools.LastIndex(g.rel, "internal")
	importIndex := pathtools.LastIndex(importPath, "internal")
	gc := getGoConfig(g.c)
	visibility := gc.goVisibility
	if relIndex >= 0 {
		parent := strings.TrimSuffix(g.rel[:relIndex], "/")
		visibility = append(visibility, fmt.Sprintf("//%s:__subpackages__", parent))
	} else if importIndex >= 0 {
		// importPath contains internal outside g.rel. Other repositories
		// under that prefix may need access.
		gc.repoConfigInputs.record(importPath[:importIndex])
		visibility = append(visibility, "//:__subpackages__")
		for _, repo := range g.c.Repos {
			if pathtools.HasPrefix(repo.AttrString("importpath"), importPath[:importIndex]) {
				visibility = append(visibility, "@"+repo.Name()+"//:__subpackages__")
			}
		}
	} else {
		return []string{"//visibility:public"}
	}

	// Add visibility for any submodules that have the internal parent as
	// a prefix of their module path.
	if importIndex >= 0 {
		gc.repoConfigInputs.record(gc.submodulesPrefix)
		internalRoot := strings.TrimSuffix(importPath[:importIndex], "/")
		for _, m := range gc.submodules {
			if strings.HasPrefix(m.modulePath, internalRoot) {
				visibility = append(visibility, fmt.Sprintf("@%s//:__subpackages__", m.repoName))
			}
		}
	}

	return visibility
}

var (
	// shortOptPrefixes are strings that come at the beginning of an option
	// argument that includes a path, e.g., -Ifoo/bar.
	shortOptPrefixes = []string{"-I", "-L", "-F"}

	// longOptPrefixes are separate arguments that come before a path argument,
	// e.g., -iquote foo/bar.
	longOptPrefixes = []string{"-I", "-L", "-F", "-iquote", "-isystem"}
)

// options transforms package-relative paths in cgo options into repository-
// root-relative paths that Bazel can understand. For example, if a cgo file
// in //foo declares an include flag in its copts: "-Ibar", this method
// will transform that flag into "-Ifoo/bar".
func (g *generator) options(opts rule.PlatformStrings, pkgRel string) rule.PlatformStrings {
	fixPath := func(opt string) string {
		if strings.HasPrefix(opt, "/") {
			return opt
		}
		return path.Clean(path.Join(pkgRel, opt))
	}

	fixGroups := func(groups []string) ([]string, error) {
		fixedGroups := make([]string, len(groups))
		for i, group := range groups {
			opts := strings.Split(group, optSeparator)
			fixedOpts := make([]string, len(opts))
			isPath := false
			for j, opt := range opts {
				if isPath {
					opt = fixPath(opt)
					isPath = false
					goto next
				}

				for _, short := range shortOptPrefixes {
					if strings.HasPrefix(opt, short) && len(opt) > len(short) {
						opt = short + fixPath(opt[len(short):])
						goto next
					}
				}

				for _, long := range longOptPrefixes {
					if opt == long {
						isPath = true
						goto next
					}
				}

			next:
				fixedOpts[j] = escapeOption(opt)
			}
			fixedGroups[i] = strings.Join(fixedOpts, " ")
		}

		return fixedGroups, nil
	}

	opts, errs := opts.MapSlice(fixGroups)
	if errs != nil {
		log.Panicf("unexpected error when transforming options with pkg %q: %v", pkgRel, errs)
	}
	return opts
}

func escapeOption(opt string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`"`, `\"`,
		` `, `\ `,
		"\t", "\\\t",
		"\n", "\\\n",
		"\r", "\\\r",
		"$(", "$(",
		"$", "$$",
	).Replace(opt)
}

func shouldSetVisibility(args language.GenerateArgs) bool {
	if args.File != nil && args.File.HasDefaultVisibility() {
		return false
	}

	for _, r := range args.OtherGen {
		// This is kind of the same test as *File.HasDefaultVisibility(),
		// but for previously defined rules.
		if r.Kind() == "package" && r.Attr("default_visibility") != nil {
			return false
		}
	}
	return true
}

func (g *generator) addRelsToIndex(ps rule.PlatformStrings) {
	// TODO: refactor to for-iterator loop after Go 1.23 is the minimum version.
	ps.Each()(func(imp string) bool {
		for _, goSearch := range g.gc.goSearch {
			if trimmed := pathtools.TrimPrefix(imp, goSearch.prefix); goSearch.prefix == "" || trimmed != imp {
				rel := path.Join(goSearch.rel, trimmed)
				if _, ok := g.relsToIndexSeen[rel]; !ok {
					g.relsToIndexSeen[rel] = struct{}{}
				}
			}
		}
		return true
	})
}
