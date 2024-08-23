package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/buildtools/build"
)

const (
	_name  = "override-generator"
	_usage = "usage: This script converts `go_repository` rules to Gazelle `go_deps` overrides to assist in the migration to Bzlmod."
)

// override kinds.
const (
	_goDepsf         = `go_deps = use_extension("%s//:extensions.bzl", "go_deps")`
	_gazelleOverride = "go_deps.gazelle_override"
	_archiveOverride = "go_deps.archive_override"
	_moduleOverride  = "go_deps.module_override"
)

// attribute constants that are used multiple times.
const (
	_buildFileGenerationAttr        = "build_file_generation"
	_buildFileProtoModeAttr         = "build_file_proto_mode"
	_patchArgsAttr                  = "patch_args"
	_buildDirectivesAttr            = "build_directives"
	_directivesAttr                 = "directives"
	_buildFileProtoModeDefault      = "default"
	_buildFileGenerationModeDefault = "auto"
)

var _mapAttrToOverride = map[string]string{
	_buildDirectivesAttr:     _gazelleOverride,
	_buildFileGenerationAttr: _gazelleOverride,
	_patchArgsAttr:           _moduleOverride,
	"patches":                _moduleOverride,
	"build_extra_args":       _gazelleOverride,
	"urls":                   _archiveOverride,
	"strip_prefix":           _archiveOverride,
	"sha256":                 _archiveOverride,
}

var _attrOverrideKeys = map[string]string{
	_buildDirectivesAttr: _directivesAttr,
}

type overrideSet map[string]*rule.Rule

type mainArgs struct {
	macroPath       string
	workspace       string
	defName         string
	outputFile      string
	gazelleRepoName string

	defaultBuildFileGeneration string
	defaultBuildFileProtoMode  string
}

func main() {
	a, err := parseArgs(os.Stderr, os.Args[1:])
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		log.Fatalf("%+v", err)
	}
	if err := run(*a, os.Stderr); err != nil {
		log.Fatal(err)
	}
}

func parseArgs(stderr io.Writer, osArgs []string) (*mainArgs, error) {
	a := &mainArgs{}
	flag := flag.NewFlagSet(_name, flag.ContinueOnError)
	flag.SetOutput(stderr)
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), _usage)
		flag.PrintDefaults()
	}
	flag.StringVar(&a.macroPath, "macro", "", "path to the macro file")
	flag.StringVar(&a.workspace, "workspace", "", "path to workspace")
	flag.StringVar(&a.defName, "def_name", "", "name of the macro definition")
	flag.StringVar(&a.outputFile, "output", "", "path to the output file")
	flag.StringVar(&a.gazelleRepoName, "gazelle_repo_name", "@bazel_gazelle", "name of the gazelle repo to load go_deps, (default: @bazel_gazelle)")
	flag.StringVar(&a.defaultBuildFileGeneration, "default_build_file_generation", "clean", "the default value for build_file_generation attribute")
	flag.StringVar(&a.defaultBuildFileProtoMode, "default_build_file_proto_mode", "default", "the default value for build_file_proto_mode attribute")
	flag.Parse(osArgs)

	if a.macroPath != "" && a.workspace != "" {
		return nil, fmt.Errorf("only one of -macro or -workspace can be specified")
	}
	if a.macroPath == "" && a.workspace == "" {
		return nil, fmt.Errorf("missing required flag: -macro or -workspace")
	}
	if a.macroPath != "" && a.defName == "" {
		return nil, fmt.Errorf("missing required flag: -def_name when -macro is specified")
	}
	if a.outputFile == "" {
		return nil, fmt.Errorf("missing required flag: -output")
	}
	return a, nil
}

func run(a mainArgs, stderr io.Writer) error {
	var w *rule.File
	var err error
	if a.macroPath != "" {
		w, err = rule.LoadMacroFile(a.macroPath, "", a.defName)
		if err != nil {
			return err
		}
	} else {
		w, err = rule.LoadWorkspaceFile(a.workspace, "")
		if err != nil {
			return err
		}
	}
	repos, _, err := repo.ListRepositories(w)
	if err != nil {
		return err
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name() < repos[j].Name()
	})

	var outputOverrides []*rule.Rule

	// Iterate over all repositories and convert them to override rules
	// The repos are ordered by "name", and the sets are sorted, so the output
	// will be deterministic.
	for _, r := range repos {
		if r.Kind() == "go_repository" {
			repoOverrides := goRepositoryToOverrideSet(r, a.defaultBuildFileGeneration, a.defaultBuildFileProtoMode)
			outputOverrides = append(outputOverrides, setToOverridesSlice(repoOverrides)...)
		}
	}

	if len(outputOverrides) == 0 {
		fmt.Fprintln(stderr, "no overrides are needed for these repos!")
		return nil
	}

	f, err := rule.LoadData(a.outputFile, "", []byte(fmt.Sprintf(_goDepsf, a.gazelleRepoName)))
	if err != nil {
		return err
	}
	for _, o := range outputOverrides {
		o.Insert(f)
	}

	if err := f.Save(a.outputFile); err != nil {
		return fmt.Errorf("error saving file: %w", err)
	}

	return nil
}

func goRepositoryToOverrideSet(r *rule.Rule, defaultBuildFileGeneration, defaultBuildFileProtoMode string) overrideSet {
	// each repo has its own override set, and can't have multiple
	// duplicate overrides. This set is created to be populated and read
	set := make(overrideSet)
	importPath := r.AttrString("importpath")

	// Load the attribute keys from the rule.
	attrs := r.AttrKeys()
	for _, attr := range attrs {
		if _, ok := _mapAttrToOverride[attr]; !ok {
			continue
		}

		attrValue := r.Attr(attr)
		
		// proto mode and build file generation require special handling.
		if attrValue == nil || attr == _buildFileProtoModeAttr || attr == _buildFileGenerationAttr {
			continue
		}

		kind := _mapAttrToOverride[attr]
		override := rule.NewRule(kind, "")
		if o, ok := set[kind]; ok {
			override = o
		}
		override.SetAttr("path", importPath)
		val := r.Attr(attr)

		// Special case for certain renamed attributes like "build_directives"
		// attribute to convert to "directives" attribute.
		if k, ok := _attrOverrideKeys[attr]; ok {
			attr = k
		}

		if val != nil {
			switch v := val.(type) {
			case *build.StringExpr:
				override.SetAttr(attr, v)
			case *build.ListExpr:
				// Special case for "patch_args" attribute to convert to
				// "patch_strip" attribute.
				if attr == _patchArgsAttr {
					setPatchArgs(r.AttrStrings(_patchArgsAttr), override)
				} else {
					override.SetAttr(attr, v)
				}
			}
		}

		set[kind] = override
	}

	// If the user default doesn't match the global default, but there's a gazelle override, we need to still apply
	// it to the individual overrides.
	// Also, since "build_file_proto_mode" is added to the "directives", we need
	// to apply it last to make sure "directives" is set.
	applyBuildFileGeneration(r, set, defaultBuildFileGeneration)
	applyBuildFileProtoMode(r, set, defaultBuildFileProtoMode, defaultBuildFileGeneration)
	return set
}

func applyBuildFileGeneration(r *rule.Rule, set overrideSet, userDefaultGeneration string) {
	ruleGeneration := r.AttrString(_buildFileGenerationAttr)
	o, ok := set[_gazelleOverride]
	if !ok {
	 	if ruleGeneration == "" || ruleGeneration == userDefaultGeneration {
			return
		}
		set[_gazelleOverride] = newGenerationOverride(r.AttrString("importpath"), ruleGeneration)
		return
	}

	if ruleGeneration == "" {
		ruleGeneration = userDefaultGeneration
	}

	o.SetAttr(_buildFileGenerationAttr, ruleGeneration)
	set[_gazelleOverride] = o
	return
}


func newGenerationOverride(path, ruleGeneration string) *rule.Rule {
	override := rule.NewRule(_gazelleOverride, "")
	override.SetAttr("path", path)
	override.SetAttr(_buildFileGenerationAttr, ruleGeneration)
	return override
}

func applyBuildFileProtoMode(r *rule.Rule, set overrideSet, userDefaultProtoMode, userDefaultGeneration string) {
	protoMode := r.AttrString(_buildFileProtoModeAttr)

	// If the gazelle_override doesn't exist. We only need to apply the proto mode
	// if it does not match the user default proto mode.
	gazelleOverride, ok := set[_gazelleOverride]
	if !ok {
		if protoMode == "" || protoMode == userDefaultProtoMode {
			return
		}
		
		set[_gazelleOverride] = newProtoOverride(r.AttrString("importpath"), protoMode)
		
		// Since it's a new override, we need to apply build_file_generation again.
		applyBuildFileGeneration(r, set, userDefaultGeneration)
		return
	}

	// If the gazelle_override exists, we should apply the override anyway since
	// the tag overwrites the defaults.
	if protoMode == "" {
		protoMode = userDefaultProtoMode
	}
	
	safeAppendDirective(gazelleOverride, "gazelle:proto " + protoMode)
	set[_gazelleOverride] = gazelleOverride
	return
}

func newProtoOverride(path, protoMode string) *rule.Rule {
	override := rule.NewRule(_gazelleOverride, "")
	override.SetAttr("path", path)
	directives := []string{"gazelle:proto " + protoMode}
	override.SetAttr(_directivesAttr, directives)
	return override
}

func safeAppendDirective(gazelleOverride *rule.Rule, directive string) {
	directives := gazelleOverride.AttrStrings(_directivesAttr)
	directiveMap := make(map[string]struct{})
	for _, d := range directives{
		directiveMap[d] = struct{}{}
	}
	if _, ok := directiveMap[directive]; ok {
		return 
	}
	directives = append(directives, directive)
	gazelleOverride.SetAttr(_directivesAttr, directives)
}

func setPatchArgs(patchArgs []string, override *rule.Rule) {
	for _, arg := range patchArgs {
		if !strings.HasPrefix(arg, "-p") {
			continue
		}
		numStr := strings.TrimPrefix(arg, "-p")
		if num, err := strconv.Atoi(numStr); err == nil {
			override.SetAttr("patch_strip", num)
			return
		}
	}
}

func setToOverridesSlice(set overrideSet) []*rule.Rule {
	// Check if both archive and module overrides exist
	if archiveOverride, archiveExists := set[_archiveOverride]; archiveExists {
		if moduleOverride, moduleExists := set[_moduleOverride]; moduleExists {
			// Merge attributes from module override into archive override
			mergeAttributes(moduleOverride, archiveOverride)
			// Remove the module override as its attributes are now merged
			delete(set, _moduleOverride)
		}
	}

	// Create a sorted slice of the remaining override keys
	var keys []string
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create a slice of overrides based on the sorted keys
	var overrides []*rule.Rule
	for _, k := range keys {
		overrides = append(overrides, set[k])
	}
	return overrides
}

func mergeAttributes(source, destination *rule.Rule) {
	for _, attr := range source.AttrKeys() {
		if val := source.Attr(attr); val != nil {
			destination.SetAttr(attr, val)
		}
	}
}
