package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/modfile"
)

const normalizedGoListTime = "0001-01-01T00:00:00Z"

var goListTimeRE = regexp.MustCompile(`"Time": "[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z"`)

type parsedModule struct {
	name        string
	version     string
	tags        tags
	tagsDev     tags
	tagsIsolate tags
	siblingDeps []string
}

func convertDirToBzl(dirPath, bzlPath string) error {
	dirPath, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(dirPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("-from path %q is not a directory", dirPath)
	}

	return withIsolatedGoEnv(func() error {
		return convertDirToBzlWithGoEnv(dirPath, bzlPath)
	})
}

func withIsolatedGoEnv(fn func() error) error {
	base, err := os.MkdirTemp("", "convert-go-deps-goenv")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base)

	home := filepath.Join(base, "home")
	gomodcache := filepath.Join(base, "gomodcache")
	gopath := filepath.Join(home, "go")
	for _, dir := range []string{home, gomodcache, gopath} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	origGOMODCACHE, err := goEnv("", "GOMODCACHE")
	if err != nil {
		return err
	}
	origGOPROXY, err := goEnv("", "GOPROXY")
	if err != nil {
		return err
	}

	env := map[string]string{
		"HOME":       home,
		"GOPATH":     gopath,
		"GOMODCACHE": gomodcache,
		"GOPROXY":    isolatedGOPROXY(origGOMODCACHE, origGOPROXY),
	}
	restore := setGoEnv(env)
	defer restore()

	return fn()
}

func isolatedGOPROXY(origGOMODCACHE, origGOPROXY string) string {
	origGOMODCACHE = filepath.Clean(origGOMODCACHE)
	if origGOMODCACHE != "" && origGOMODCACHE != "." && filepath.IsAbs(origGOMODCACHE) {
		return fileProxyURL(origGOMODCACHE) + "," + origGOPROXY
	}
	return origGOPROXY
}

func fileProxyURL(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(path))}
	return u.String()
}

func setGoEnv(values map[string]string) func() {
	orig := map[string]string{}
	had := map[string]bool{}
	for key, value := range values {
		if v, ok := os.LookupEnv(key); ok {
			orig[key] = v
			had[key] = true
		}
		os.Setenv(key, value)
	}
	return func() {
		for key := range values {
			if had[key] {
				os.Setenv(key, orig[key])
			} else {
				os.Unsetenv(key)
			}
		}
	}
}

func convertDirToBzlWithGoEnv(dirPath, bzlPath string) error {
	docstring, err := os.ReadFile(filepath.Join(dirPath, "comment.txt"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	parsed, err := parseModuleDirs(dirPath)
	if err != nil {
		return err
	}
	if len(parsed) == 0 {
		return fmt.Errorf("%s: no module directories with MODULE.bazel found", dirPath)
	}

	rootName, err := findRootModule(parsed)
	if err != nil {
		return err
	}

	moduleNames := make(map[string]bool, len(parsed))
	for name := range parsed {
		moduleNames[name] = true
	}

	files, err := collectModuleFiles(dirPath, moduleNames)
	if err != nil {
		return err
	}

	want, err := readWantJSON(dirPath)
	if err != nil {
		return err
	}

	testName := strings.TrimSuffix(filepath.Base(bzlPath), ".bzl")
	tcForWork := testCaseFromParsed(testName, parsed, rootName, files)
	if err := writeGoDepsWorkFiles(dirPath, tcForWork); err != nil {
		return fmt.Errorf("write go_deps convenience files: %w", err)
	}

	executions := map[string]map[string]string{}
	mainExecs, err := deriveExecutions(dirPath, files, goDepsWorkDir("", false), collectFromFileTags(parsed, false))
	if err != nil {
		return err
	}
	if len(mainExecs) > 0 {
		executions[mainExecutionKey] = mainExecs
	}
	for name, pm := range parsed {
		if pm.tagsIsolate.isEmpty() {
			continue
		}
		isolateExecs, err := deriveExecutions(dirPath, files, goDepsWorkDir(name, true), collectFromFileTagsForModule(pm))
		if err != nil {
			return err
		}
		if len(isolateExecs) > 0 {
			executions[isolateExecutionKey(name)] = isolateExecs
		}
	}

	modules := make([]module, 0, len(parsed))
	names := make([]string, 0, len(parsed)-1)
	for name := range parsed {
		if name != rootName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	names = append([]string{rootName}, names...)
	for _, name := range names {
		pm := parsed[name]
		m := module{
			Name:    pm.name,
			IsRoot:  name == rootName,
			Version: pm.version,
		}
		if !pm.tags.isEmpty() {
			t := pm.tags
			m.Tags = &t
		}
		if !pm.tagsDev.isEmpty() {
			t := pm.tagsDev
			m.TagsDev = &t
		}
		if !pm.tagsIsolate.isEmpty() {
			t := pm.tagsIsolate
			m.TagsIsolate = &t
		}
		modules = append(modules, m)
	}

	tc := testCase{
		Name:       testName,
		Modules:    modules,
		Files:      files,
		Executions: executions,
		Want:       want,
	}

	content, err := renderTestCaseBzl(string(docstring), &tc)
	if err != nil {
		return err
	}

	return os.WriteFile(bzlPath, content, 0o644)
}

func parseModuleDirs(dirPath string) (map[string]*parsedModule, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	parsed := map[string]*parsedModule{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		modPath := filepath.Join(dirPath, entry.Name(), "MODULE.bazel")
		data, err := os.ReadFile(modPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		pm, err := parseModuleBazel(entry.Name(), data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", modPath, err)
		}
		parsed[entry.Name()] = pm
	}
	return parsed, nil
}

func parseModuleBazel(dirName string, data []byte) (*parsedModule, error) {
	file, err := build.ParseModule("MODULE.bazel", data)
	if err != nil {
		return nil, err
	}

	pm := &parsedModule{name: dirName}

	for _, stmt := range file.Stmt {
		call, ok := stmt.(*build.CallExpr)
		if !ok {
			continue
		}

		if tagType, target, ok := goDepsTagTargetForModule(call, pm); ok {
			attrs, err := callAttrs(call)
			if err != nil {
				return nil, err
			}
			attrs = labelAttrsForTestCase(pm.name, attrs)
			attrs = omitTagDefaults(tagType, attrs)
			if err := appendTag(target, tagType, attrs); err != nil {
				return nil, err
			}
			continue
		}

		ident, ok := call.X.(*build.Ident)
		if !ok {
			continue
		}
		switch ident.Name {
		case "module":
			attrs, err := callAttrs(call)
			if err != nil {
				return nil, err
			}
			if name, ok := attrs["name"].(string); ok {
				pm.name = name
			}
			if version, ok := attrs["version"].(string); ok {
				pm.version = version
			}
		case "local_path_override":
			attrs, err := callAttrs(call)
			if err != nil {
				return nil, err
			}
			moduleName, _ := attrs["module_name"].(string)
			path, _ := attrs["path"].(string)
			if moduleName != "" && moduleName != "gazelle" && strings.HasPrefix(path, "../") {
				pm.siblingDeps = append(pm.siblingDeps, moduleName)
			}
		}
	}

	sort.Strings(pm.siblingDeps)
	return pm, nil
}

func goDepsTagTargetForModule(call *build.CallExpr, pm *parsedModule) (tagType string, target *tags, ok bool) {
	dot, ok := call.X.(*build.DotExpr)
	if !ok {
		return "", nil, false
	}
	ident, ok := dot.X.(*build.Ident)
	if !ok {
		return "", nil, false
	}
	switch ident.Name {
	case "go_deps":
		return dot.Name, &pm.tags, true
	case "go_deps_dev":
		return dot.Name, &pm.tagsDev, true
	case "go_deps_isolate":
		return dot.Name, &pm.tagsIsolate, true
	default:
		return "", nil, false
	}
}

func callAttrs(call *build.CallExpr) (map[string]any, error) {
	attrs := map[string]any{}
	for _, arg := range call.List {
		switch a := arg.(type) {
		case *build.AssignExpr:
			lhs, ok := a.LHS.(*build.Ident)
			if !ok {
				continue
			}
			value, err := exprToValue(a.RHS)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", lhs.Name, err)
			}
			attrs[lhs.Name] = value
		case *build.Ident:
			// positional arg, ignore
		}
	}
	return attrs, nil
}

func exprToValue(e build.Expr) (any, error) {
	switch v := e.(type) {
	case *build.StringExpr:
		return v.Value, nil
	case *build.Ident:
		switch v.Name {
		case "True":
			return true, nil
		case "False":
			return false, nil
		case "None":
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported ident %q", v.Name)
		}
	case *build.LiteralExpr:
		if i, err := strconv.ParseInt(v.Token, 10, 64); err == nil {
			return i, nil
		}
		if f, err := strconv.ParseFloat(v.Token, 64); err == nil {
			return f, nil
		}
		return v.Token, nil
	case *build.UnaryExpr:
		if v.Op == "-" {
			if lit, ok := v.X.(*build.LiteralExpr); ok {
				if i, err := strconv.ParseInt(lit.Token, 10, 64); err == nil {
					return -i, nil
				}
			}
		}
		return nil, fmt.Errorf("unsupported unary expression")
	case *build.ListExpr:
		out := make([]any, 0, len(v.List))
		for _, item := range v.List {
			val, err := exprToValue(item)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
		return out, nil
	case *build.DictExpr:
		out := map[string]any{}
		for _, item := range v.List {
			key, err := dictKey(item.Key)
			if err != nil {
				return nil, err
			}
			val, err := exprToValue(item.Value)
			if err != nil {
				return nil, err
			}
			out[key] = val
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported expression type %T", e)
	}
}

func dictKey(e build.Expr) (string, error) {
	switch k := e.(type) {
	case *build.StringExpr:
		return k.Value, nil
	case *build.Ident:
		return k.Name, nil
	default:
		return "", fmt.Errorf("unsupported dict key type %T", e)
	}
}

func appendTag(t *tags, tagType string, attrs map[string]any) error {
	if len(attrs) == 0 {
		attrs = map[string]any{}
	}
	switch tagType {
	case "archive_override":
		t.ArchiveOverride = append(t.ArchiveOverride, attrs)
	case "config":
		t.Config = append(t.Config, attrs)
	case "from_file":
		t.FromFile = append(t.FromFile, attrs)
	case "gazelle_override":
		t.GazelleOverride = append(t.GazelleOverride, attrs)
	case "gazelle_default_attributes":
		t.GazelleDefaultAttributes = append(t.GazelleDefaultAttributes, attrs)
	case "module":
		t.Module = append(t.Module, attrs)
	case "module_override":
		t.ModuleOverride = append(t.ModuleOverride, attrs)
	default:
		// ignore use_repo and unknown tags
	}
	return nil
}

func labelAttrsForTestCase(moduleName string, attrs map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range attrs {
		if key == "go_mod" || key == "go_work" {
			if s, ok := value.(string); ok {
				out[key] = labelForTestCase(s, moduleName)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func labelForTestCase(label, moduleName string) string {
	if strings.HasPrefix(label, "//") {
		return "@@" + moduleName + label
	}
	return label
}

func omitTagDefaults(tagType string, attrs map[string]any) map[string]any {
	defaults := tagDefaults[tagType]
	if len(defaults) == 0 {
		return attrs
	}
	out := map[string]any{}
	for key, value := range attrs {
		if defaultValue, ok := defaults[key]; ok && valuesEqual(value, defaultValue) {
			continue
		}
		out[key] = value
	}
	return out
}

var tagDefaults = map[string]map[string]any{
	"config": {
		"check_direct_dependencies": "off",
		"debug_mode":                false,
		"go_env":                    map[string]any{},
		"go_env_inherit":            []any{},
	},
	"from_file": {
		"fail_on_version_conflict": true,
	},
	"gazelle_default_attributes": {
		"build_file_generation": "on",
		"build_extra_args":      []any{},
		"directives":            []any{},
	},
	"gazelle_override": {
		"build_file_generation": "on",
		"build_extra_args":      []any{},
		"directives":            []any{},
	},
	"module": {
		"sum":                     "",
		"indirect":                false,
		"build_naming_convention": "",
		"build_file_proto_mode":   "",
		"local_path":              "",
	},
	"module_override": {
		"patches":     []any{},
		"patch_strip": []any{},
		"patch_cmds":  []any{},
		"repo_name":   "",
	},
	"archive_override": {
		"urls":         []any{},
		"strip_prefix": "",
		"sha256":       "",
		"patches":      []any{},
		"patch_strip":  0,
		"patch_cmds":   []any{},
	},
}

func valuesEqual(a, b any) bool {
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
}

func (t tags) isEmpty() bool {
	return len(t.ArchiveOverride) == 0 &&
		len(t.Config) == 0 &&
		len(t.FromFile) == 0 &&
		len(t.GazelleOverride) == 0 &&
		len(t.GazelleDefaultAttributes) == 0 &&
		len(t.Module) == 0 &&
		len(t.ModuleOverride) == 0
}

func findRootModule(parsed map[string]*parsedModule) (string, error) {
	referenced := map[string]bool{}
	for _, pm := range parsed {
		for _, dep := range pm.siblingDeps {
			referenced[dep] = true
		}
	}

	var roots []string
	for name := range parsed {
		if !referenced[name] {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)

	for _, name := range roots {
		if len(parsed[name].siblingDeps) > 0 {
			return name, nil
		}
	}
	if len(roots) == 1 {
		return roots[0], nil
	}
	return "", fmt.Errorf("could not determine root module among %v", roots)
}

func testCaseFromParsed(testName string, parsed map[string]*parsedModule, rootName string, files map[string]string) *testCase {
	names := make([]string, 0, len(parsed)-1)
	for name := range parsed {
		if name != rootName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	names = append([]string{rootName}, names...)

	modules := make([]module, 0, len(parsed))
	for _, name := range names {
		pm := parsed[name]
		m := module{
			Name:    pm.name,
			IsRoot:  name == rootName,
			Version: pm.version,
		}
		if !pm.tags.isEmpty() {
			t := pm.tags
			m.Tags = &t
		}
		if !pm.tagsDev.isEmpty() {
			t := pm.tagsDev
			m.TagsDev = &t
		}
		if !pm.tagsIsolate.isEmpty() {
			t := pm.tagsIsolate
			m.TagsIsolate = &t
		}
		modules = append(modules, m)
	}
	return &testCase{
		Name:    testName,
		Modules: modules,
		Files:   files,
	}
}

func collectFromFileTags(parsed map[string]*parsedModule, isolated bool) []fromFileRef {
	var refs []fromFileRef
	for name, pm := range parsed {
		var tagSets []tags
		if isolated {
			tagSets = []tags{pm.tagsIsolate}
		} else {
			tagSets = []tags{pm.tags, pm.tagsDev}
		}
		for _, ts := range tagSets {
			for _, tag := range ts.FromFile {
				refs = append(refs, fromFileRef{
					moduleName: name,
					tag:        tag,
				})
			}
		}
	}
	return refs
}

func collectFromFileTagsForModule(pm *parsedModule) []fromFileRef {
	var refs []fromFileRef
	for _, tag := range pm.tagsIsolate.FromFile {
		refs = append(refs, fromFileRef{moduleName: pm.name, tag: tag})
	}
	return refs
}

type fromFileRef struct {
	moduleName string
	tag        map[string]any
}

func expandFromFileRefs(files map[string]string, fromFileRefs []fromFileRef) ([]fromFileRef, error) {
	expanded := append([]fromFileRef(nil), fromFileRefs...)
	seenGoMod := map[string]bool{}
	for _, ref := range fromFileRefs {
		goWork, hasGoWork := ref.tag["go_work"].(string)
		if !hasGoWork || goWork == "" {
			continue
		}
		fileKey, err := labelToFileKey(goWork)
		if err != nil {
			return nil, err
		}
		content, ok := files[fileKey]
		if !ok {
			return nil, fmt.Errorf("missing go.work content for label %q (expected files[%q])", goWork, fileKey)
		}
		wf, err := modfile.ParseWork("go.work", []byte(content), nil)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", fileKey, err)
		}
		for _, u := range wf.Use {
			if !isRelativeUsePath(u.Path) {
				continue
			}
			goModLabel, err := goModLabelFromGoWork(goWork, u.Path)
			if err != nil {
				return nil, err
			}
			goModPath, err := labelToFileKey(goModLabel)
			if err != nil {
				return nil, err
			}
			if seenGoMod[goModPath] {
				continue
			}
			seenGoMod[goModPath] = true
			expanded = append(expanded, fromFileRef{
				moduleName: ref.moduleName,
				tag:        map[string]any{"go_mod": goModLabel},
			})
		}
	}
	return expanded, nil
}

func deriveExecutions(dirPath string, files map[string]string, workSubdir string, fromFileRefs []fromFileRef) (map[string]string, error) {
	executions := map[string]string{}
	expandedRefs, err := expandFromFileRefs(files, fromFileRefs)
	if err != nil {
		return nil, err
	}
	if err := appendDerivedGoEditExecutions(dirPath, files, expandedRefs, executions); err != nil {
		return nil, err
	}
	workDir := filepath.Join(dirPath, workSubdir)
	if err := appendGoListExecution(workDir, dirPath, executions); err != nil {
		return nil, err
	}
	return executions, nil
}

func appendDerivedGoEditExecutions(dirPath string, files map[string]string, fromFileRefs []fromFileRef, executions map[string]string) error {
	seen := map[string]bool{}
	for _, ref := range fromFileRefs {
		goMod, hasGoMod := ref.tag["go_mod"].(string)
		goWork, hasGoWork := ref.tag["go_work"].(string)
		switch {
		case hasGoMod && goMod != "":
			path, err := labelToFileKey(goMod)
			if err != nil {
				return err
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			command := "go mod edit -json -- " + path
			stdout, err := runGoEditCommand(dirPath, "mod", path)
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			executions[command] = stdout
		case hasGoWork && goWork != "":
			path, err := labelToFileKey(goWork)
			if err != nil {
				return err
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			command := "go work edit -json -- " + path
			stdout, err := runGoEditCommand(dirPath, "work", path)
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			executions[command] = stdout
		}
	}
	return nil
}

func appendGoListExecution(workDir, normalizeDir string, executions map[string]string) error {
	goWork := filepath.Join(workDir, "go.work")
	if _, err := os.Stat(goWork); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := syncGoWorkVersion(workDir); err != nil {
		return fmt.Errorf("%s: %w", goListCommand, err)
	}
	stdout, err := runGoListCommand(workDir)
	if err != nil {
		// Some test cases expect go_deps to fail; go list may not succeed either.
		return nil
	}
	executions[goListCommand] = normalizeGoListOutput(normalizeDir, stdout)
	return nil
}

func syncGoWorkVersion(dirPath string) error {
	maxVersion, err := maxGoModVersion(dirPath)
	if err != nil || maxVersion == "" {
		return err
	}
	cmd, err := goCommand(dirPath, "work", "edit", "-go="+maxVersion)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func maxGoModVersion(dirPath string) (string, error) {
	var maxVersion string
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "go.mod" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(line, "go "); ok {
				if goVersionLess(maxVersion, after) {
					maxVersion = after
				}
				break
			}
		}
		return nil
	})
	return maxVersion, err
}

func goVersionLess(a, b string) bool {
	if a == "" {
		return true
	}
	return compareGoVersions(a, b) < 0
}

func compareGoVersions(a, b string) int {
	ap := parseGoVersion(a)
	bp := parseGoVersion(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func parseGoVersion(version string) [3]int {
	var parts [3]int
	for i, part := range strings.Split(version, ".") {
		if i >= 3 {
			break
		}
		parts[i], _ = strconv.Atoi(part)
	}
	return parts
}

func runGoListCommand(dirPath string) (string, error) {
	cmd, err := goCommand(dirPath, "list", "-m", "-json", "all")
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

func goEnv(dirPath, key string) (string, error) {
	cmd, err := goCommand(dirPath, "env", key)
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func normalizeGoListOutput(dirPath, stdout string) string {
	stdout = normalizeTestDirPaths(dirPath, stdout)
	return goListTimeRE.ReplaceAllString(stdout, `"Time": "`+normalizedGoListTime+`"`)
}

func normalizeTestDirPaths(dirPath, stdout string) string {
	dirPath = abs(dirPath)
	stdout = strings.ReplaceAll(stdout, `\\`, "/")
	stdout = strings.ReplaceAll(stdout, dirPath, "/test")
	if eval, err := filepath.EvalSymlinks(dirPath); err == nil && eval != dirPath {
		stdout = strings.ReplaceAll(stdout, eval, "/test")
	}
	if gomodcache, err := goEnv(dirPath, "GOMODCACHE"); err == nil && gomodcache != "" {
		gomodcache = abs(gomodcache)
		stdout = strings.ReplaceAll(stdout, gomodcache, "/gomodcache")
	}
	return stdout
}

func abs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(p)
}

func runGoEditCommand(dirPath, subcommand, path string) (string, error) {
	cmd, err := goCommand(dirPath, subcommand, "edit", "-json", "--", path)
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

func readWantJSON(dirPath string) (map[string]json.RawMessage, error) {
	path := filepath.Join(dirPath, "want.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s: invalid JSON", path)
	}
	var want map[string]json.RawMessage
	if err := json.Unmarshal(data, &want); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return want, nil
}

func collectModuleFiles(dirPath string, moduleNames map[string]bool) (map[string]string, error) {
	files := map[string]string{}
	for modName := range moduleNames {
		modDir := filepath.Join(dirPath, modName)
		err := filepath.WalkDir(modDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			base := d.Name()
			if base == "MODULE.bazel" || base == "MODULE.bazel.lock" || base == "BUILD" {
				return nil
			}
			rel, err := filepath.Rel(modDir, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			key := "./" + modName + "/" + filepath.ToSlash(rel)
			files[key] = string(data)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

const generatedTestCaseHeader = `# Generated by //tools/convert-go-deps-test-case. DO NOT EDIT.
# See README.md for instructions.

`

func renderTestCaseBzl(docstring string, tc *testCase) ([]byte, error) {
	jsonBytes, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString(generatedTestCaseHeader)
	if docstring != "" {
		b.WriteString(`"""`)
		b.WriteString("\n")
		b.WriteString(docstring)
		b.WriteString(`"""`)
		b.WriteString("\n\n")
	}
	b.WriteString("TEST = r\"\"\"\n")
	b.Write(jsonBytes)
	b.WriteString("\n\"\"\"\n")
	return []byte(b.String()), nil
}
