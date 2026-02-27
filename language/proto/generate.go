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

package proto

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/merger"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (*protoLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	c := args.Config
	pc := GetProtoConfig(c)
	if !pc.Mode.ShouldGenerateRules() {
		// Don't create or delete proto rules in this mode. Any existing rules
		// are likely hand-written.
		return language.GenerateResult{}
	}

	var regularProtoFiles []string
	for _, name := range args.RegularFiles {
		if strings.HasSuffix(name, ".proto") {
			regularProtoFiles = append(regularProtoFiles, name)
		}
	}

	// Some of the generated files may have been consumed by other rules
	consumedFileSet := make(map[string]bool)
	for _, r := range args.OtherGen {
		if r.Kind() != "proto_library" {
			continue
		}
		for _, f := range r.AttrStrings("srcs") {
			consumedFileSet[f] = true
		}
	}

	// genProtoFilesNotConsumed represents only not consumed generated files.
	// genProtoFiles represents all generated files.
	// This is required for not generating empty rules for consumed generated
	// files.
	var genProtoFiles, genProtoFilesNotConsumed []string
	for _, name := range args.GenFiles {
		if strings.HasSuffix(name, ".proto") {
			genProtoFiles = append(genProtoFiles, name)
			if !consumedFileSet[name] {
				genProtoFilesNotConsumed = append(genProtoFilesNotConsumed, name)
			}
		}
	}
	pkgs := buildPackages(pc, args.Dir, args.Rel, regularProtoFiles, genProtoFilesNotConsumed)
	shouldSetVisibility := args.File == nil || !args.File.HasDefaultVisibility()
	var res language.GenerateResult
	for _, pkg := range pkgs {
		r := generateProto(pc, args.Rel, pkg, shouldSetVisibility)
		if args.File != nil {
			// If matching rule already exists, use its name for generated rule, otherwise other languages may not be able to resolve proto_library rule.
			// This way we can propagate the name that would actually written to the BUILD file.
			// Most of downstream extensions would refer to this name directly when generating `<lang>_proto_library`.
			previous, err := merger.Match(args.File.Rules, r, protoKinds["proto_library"], c.AliasMap)
			if err == nil && previous != nil {
				r.SetName(previous.Name())
			}
		}
		if r.IsEmpty(protoKinds[r.Kind()]) {
			res.Empty = append(res.Empty, r)
		} else {
			res.Gen = append(res.Gen, r)
		}
	}
	sort.SliceStable(res.Gen, func(i, j int) bool {
		return res.Gen[i].Name() < res.Gen[j].Name()
	})
	res.Imports = make([]interface{}, len(res.Gen))
	for i, r := range res.Gen {
		res.Imports[i] = r.PrivateAttr(config.GazelleImportsKey)
	}
	res.Empty = append(res.Empty, generateEmpty(args.File, regularProtoFiles, genProtoFiles)...)
	res.RelsToIndex = buildRelsToIndex(pc, pkgs)
	return res
}

// RuleName returns a name for a proto_library derived from the given strings.
// For each string, RuleName will look for a non-empty suffix of identifier
// characters and then append "_proto" to that.
// It replaces non-identifier characters with underscores.
func RuleName(names ...string) string {
	for _, name := range names {
		notIdent := func(c rune) bool {
			return !('A' <= c && c <= 'Z' ||
				'a' <= c && c <= 'z' ||
				'0' <= c && c <= '9' ||
				c == '_')
		}
		// If name is explicit package name, e.g. `example.com/protos/foo;package_name` use package name instead of import path
		if i := strings.LastIndexAny(name, `;`); i != -1 {
			name = name[i+1:]
		}
		// If name is a path, take only the last segment
		if i := strings.LastIndexAny(name, `/\\.`); i != -1 {
			name = name[i+1:]
		}
		// Replace illegal characters with underscores
		var b strings.Builder
		for _, r := range name {
			if notIdent(r) {
				b.WriteRune('_')
			} else {
				b.WriteRune(r)
			}
		}
		base := strings.Trim(b.String(), "_")
		// Skip if empty or only underscores
		if base != "" {
			return base + "_proto"
		}
	}
	// Default name if no valid identifier was found
	return "root_proto"
}

// buildPackage extracts metadata from the .proto files in a directory and
// constructs possibly several packages, then selects a package to generate
// a proto_library rule for.
func buildPackages(pc *ProtoConfig, dir, rel string, protoFiles, genFiles []string) []*Package {
	packageMap := make(map[string]*Package)
	fileMap := make(map[string]FileInfo)

	for _, name := range protoFiles {
		info := ProtoFileInfo(dir, name)
		fileMap[name] = info
		key := info.PackageName

		if pc.Mode == FileMode {
			key = strings.TrimSuffix(name, ".proto")
		} else if pc.groupOption != "" { // implicitly PackageMode
			for _, opt := range info.Options {
				if opt.Key == pc.groupOption {
					key = opt.Value
					break
				}
			}
		}

		if packageMap[key] == nil {
			packageMap[key] = newPackage(info.PackageName)
		}
		packageMap[key].addFile(info)
		if key != info.PackageName {
			packageMap[key].RuleName = key
		}
	}

	if pc.fileIncludeOption != "" {
		for _, name := range strings.Fields(pc.fileIncludeOption) {
			wantFile, ok := fileMap[name]
			if !ok {
				log.Printf("error in package %s: the requested file %q named in the 'gazelle:proto_file_include' option was not found'", rel, name)
				continue
			}
			for _, pkg := range packageMap {
				if _, ok := pkg.Files[name]; ok {
					continue
				}
				pkg.addFile(wantFile)
			}
		}
	}

	switch pc.Mode {
	case DefaultMode:
		pkg, err := selectPackage(dir, rel, packageMap)
		if err != nil {
			log.Print(err)
		}
		if pkg == nil {
			return nil // empty rule created in generateEmpty
		}
		for _, name := range genFiles {
			pkg.addGenFile(dir, name)
		}
		return []*Package{pkg}

	case PackageMode, FileMode:
		pkgs := make([]*Package, 0, len(packageMap))
		for _, pkg := range packageMap {
			pkgs = append(pkgs, pkg)
		}
		return pkgs

	default:
		return nil
	}
}

// selectPackage chooses a package to generate rules for.
func selectPackage(dir, rel string, packageMap map[string]*Package) (*Package, error) {
	if len(packageMap) == 0 {
		return nil, nil
	}
	if len(packageMap) == 1 {
		for _, pkg := range packageMap {
			return pkg, nil
		}
	}
	defaultPackageName := strings.Replace(rel, "/", "_", -1)
	for _, pkg := range packageMap {
		if pkgName := goPackageName(pkg); pkgName != "" && pkgName == defaultPackageName {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("%s: directory contains multiple proto packages. Gazelle can only generate a proto_library for one package.", dir)
}

// goPackageName guesses the identifier in package declarations at the top of
// the .pb.go files that will be generated for this package. "" is returned
// if the package name cannot be determined.
//
// TODO(jayconrod): remove all Go-specific functionality. This is here
// temporarily for compatibility.
func goPackageName(pkg *Package) string {
	if opt, ok := pkg.Options["go_package"]; ok {
		if i := strings.IndexByte(opt, ';'); i >= 0 {
			return opt[i+1:]
		} else if i := strings.LastIndexByte(opt, '/'); i >= 0 {
			return opt[i+1:]
		} else {
			return opt
		}
	}
	if pkg.Name != "" {
		return strings.Replace(pkg.Name, ".", "_", -1)
	}
	if len(pkg.Files) == 1 {
		for s := range pkg.Files {
			return strings.TrimSuffix(s, ".proto")
		}
	}
	return ""
}

// generateProto creates a new proto_library rule for a package. The rule may
// be empty if there are no sources.
func generateProto(pc *ProtoConfig, rel string, pkg *Package, shouldSetVisibility bool) *rule.Rule {
	var name string
	if pc.Mode == DefaultMode {
		name = RuleName(goPackageName(pkg), pc.GoPrefix, rel)
	} else {
		name = RuleName(pkg.RuleName, pkg.Name, rel)
	}
	r := rule.NewRule("proto_library", name)
	srcs := make([]string, 0, len(pkg.Files))
	for f := range pkg.Files {
		srcs = append(srcs, f)
	}
	sort.Strings(srcs)
	if len(srcs) > 0 {
		r.SetAttr("srcs", srcs)
	}
	r.SetPrivateAttr(PackageKey, *pkg)
	imports := make([]string, 0, len(pkg.Imports))
	for i := range pkg.Imports {
		// If the proto import is a self import (an import between the same package), skip it
		if _, ok := pkg.Files[path.Base(i)]; ok && getPrefix(pc, path.Dir(i)) == getPrefix(pc, rel) {
			delete(pkg.Imports, i)
			continue
		}
		imports = append(imports, i)
	}
	sort.Strings(imports)
	// NOTE: This attribute should not be used outside this extension. It's still
	// convenient for testing though.
	r.SetPrivateAttr(config.GazelleImportsKey, imports)
	for k, v := range pkg.Options {
		r.SetPrivateAttr(k, v)
	}
	if shouldSetVisibility {
		vis := rule.CheckInternalVisibility(rel, "//visibility:public")
		r.SetAttr("visibility", []string{vis})
	}
	if pc.StripImportPrefix != "" {
		r.SetAttr("strip_import_prefix", pc.StripImportPrefix)
	}
	if pc.ImportPrefix != "" {
		r.SetAttr("import_prefix", pc.ImportPrefix)
	}
	return r
}

func getPrefix(pc *ProtoConfig, rel string) string {
	prefix := rel
	if strings.HasPrefix(pc.StripImportPrefix, "/") {
		prefix = pathtools.TrimPrefix(rel, pc.StripImportPrefix[len("/"):])
	} else if pc.StripImportPrefix != "" {
		prefix = pathtools.TrimPrefix(rel, path.Join(rel, pc.StripImportPrefix))
	}
	if pc.ImportPrefix != "" {
		return path.Join(pc.ImportPrefix, prefix)
	}
	return prefix
}

// generateEmpty generates a list of proto_library rules that may be deleted.
// This is generated from existing proto_library rules with srcs lists that
// don't match any static or generated files.
func generateEmpty(f *rule.File, regularFiles, genFiles []string) []*rule.Rule {
	if f == nil {
		return nil
	}
	knownFiles := make(map[string]bool)
	for _, f := range regularFiles {
		knownFiles[f] = true
	}
	for _, f := range genFiles {
		knownFiles[f] = true
	}
	var empty []*rule.Rule
outer:
	for _, r := range f.Rules {
		if r.Kind() != "proto_library" {
			continue
		}
		srcs := r.AttrStrings("srcs")
		if len(srcs) == 0 && r.Attr("srcs") != nil {
			// srcs is not a string list; leave it alone
			continue
		}
		for _, src := range r.AttrStrings("srcs") {
			if knownFiles[src] {
				continue outer
			}
		}
		empty = append(empty, rule.NewRule("proto_library", r.Name()))
	}
	return empty
}

// buildRelsToIndex transforms the import statements read from proto files in the current directory
// into a list of repo-root-relative directory paths to lazily index.
func buildRelsToIndex(pc *ProtoConfig, packages []*Package) []string {
	dirSet := make(map[string]bool)

	for _, pkg := range packages {
		for importStr := range pkg.Imports {
			for _, search := range pc.protoSearch {
				transformedRel, ok := transformImport("", importStr, search.stripImportPrefix, search.importPrefix)
				if !ok {
					continue
				}
				dir := path.Dir(transformedRel)
				if dir == "." {
					dir = ""
				}
				dirSet[dir] = true
			}
		}
	}

	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	return dirs
}
