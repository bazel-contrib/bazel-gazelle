package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"golang.org/x/mod/modfile"
)

func convertBzlToDir(bzlPath, dirPath string, force bool, repoRoot string) error {
	tc, docstring, err := parseTestCaseFile(bzlPath)
	if err != nil {
		return err
	}

	if force {
		if err := os.RemoveAll(dirPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := prepareOutputDir(dirPath); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dirPath, "comment.txt"), []byte(docstring), 0666); err != nil {
		return err
	}

	if err := writeWantJSON(dirPath, tc.Want); err != nil {
		return err
	}

	if err := writeFiles(dirPath, tc.Files); err != nil {
		return err
	}

	var root *module
	others := make([]module, 0, len(tc.Modules)-1)
	for i := range tc.Modules {
		m := &tc.Modules[i]
		if m.IsRoot {
			root = m
		} else {
			others = append(others, *m)
		}
	}
	if root == nil {
		return fmt.Errorf("test case has no module with is_root = true")
	}

	for i := range tc.Modules {
		m := &tc.Modules[i]
		var deps []module
		if m.IsRoot {
			deps = others
		}
		file, err := buildModuleBazelFile(m, deps, repoRoot)
		if err != nil {
			return fmt.Errorf("render MODULE.bazel for %s: %w", m.Name, err)
		}
		modDir := filepath.Join(dirPath, m.Name)
		if err := os.MkdirAll(modDir, 0777); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(modDir, "MODULE.bazel"), build.Format(file), 0666); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(modDir, "BUILD"), nil, 0666); err != nil {
			return err
		}
	}

	if err := writeGoDepsWorkFiles(dirPath, tc); err != nil {
		return fmt.Errorf("write go_deps convenience files: %w", err)
	}

	return nil
}

func writeWantJSON(dirPath string, want map[string]json.RawMessage) error {
	if len(want) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		return fmt.Errorf("encode want.json: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dirPath, "want.json"), data, 0666)
}

func parseTestCaseFile(path string) (*testCase, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	file, err := build.ParseBzl(path, data)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}

	var docstring string
	var jsonText string
	for _, stmt := range file.Stmt {
		if assign, ok := stmt.(*build.AssignExpr); ok {
			ident, ok := assign.LHS.(*build.Ident)
			if !ok || ident.Name != "TEST" {
				continue
			}
			str, ok := assign.RHS.(*build.StringExpr)
			if !ok {
				return nil, "", fmt.Errorf("%s: TEST must be assigned a string literal", path)
			}
			jsonText = str.Value
			break
		}
		if docstring == "" {
			if str, ok := stmt.(*build.StringExpr); ok {
				docstring = strings.TrimLeft(str.Value, " \t\r\n")
			}
		}
	}
	if jsonText == "" {
		return nil, "", fmt.Errorf("%s: missing TEST string assignment", path)
	}

	var tc testCase
	if err := json.Unmarshal([]byte(jsonText), &tc); err != nil {
		return nil, "", fmt.Errorf("decode test case JSON: %w", err)
	}
	return &tc, docstring, nil
}

func prepareOutputDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0777)
		}
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("-to directory %s must be empty", dir)
	}
	return nil
}

func writeFiles(outDir string, files map[string]string) error {
	for key, content := range files {
		path := filepath.Join(outDir, key)
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0666); err != nil {
			return err
		}
	}
	return nil
}

func tagsNonempty(t *tags) bool {
	if t == nil {
		return false
	}
	return len(t.ArchiveOverride) > 0 ||
		len(t.Config) > 0 ||
		len(t.FromFile) > 0 ||
		len(t.GazelleOverride) > 0 ||
		len(t.GazelleDefaultAttributes) > 0 ||
		len(t.Module) > 0 ||
		len(t.ModuleOverride) > 0
}

func buildModuleBazelFile(m *module, deps []module, gazelleRoot string) (*build.File, error) {
	var stmts []build.Expr

	moduleArgs := []build.Expr{bazelAssign("name", stringExpr(m.Name))}
	if m.Version != "" {
		moduleArgs = append(moduleArgs, bazelAssign("version", stringExpr(m.Version)))
	}
	stmts = append(stmts, bazelCall("module", moduleArgs...))

	stmts = append(stmts,
		bazelCall("bazel_dep",
			bazelAssign("name", stringExpr("gazelle")),
			bazelAssign("version", stringExpr("1.0.0")),
		),
		bazelCall("local_path_override",
			bazelAssign("module_name", stringExpr("gazelle")),
			bazelAssign("path", stringExpr(gazelleRoot)),
		),
	)

	for _, dep := range deps {
		version := dep.Version
		if version == "" {
			version = "1.0.0"
		}
		stmts = append(stmts,
			bazelCall("bazel_dep",
				bazelAssign("name", stringExpr(dep.Name)),
				bazelAssign("version", stringExpr(version)),
			),
			bazelCall("local_path_override",
				bazelAssign("module_name", stringExpr(dep.Name)),
				bazelAssign("path", stringExpr("../"+dep.Name)),
			),
		)
	}

	stmts = append(stmts, &build.AssignExpr{
		LHS: bazelIdent("go_deps"),
		Op:  "=",
		RHS: bazelCall("use_extension",
			stringExpr("@gazelle//:extensions.bzl"),
			stringExpr("go_deps"),
		),
	})

	modTags := tags{}
	if m.Tags != nil {
		modTags = *m.Tags
	}
	tagStmts, err := goDepsTagStmts("go_deps", modTags, m.Name)
	if err != nil {
		return nil, err
	}
	stmts = append(stmts, tagStmts...)

	if tagsNonempty(m.TagsDev) {
		stmts = append(stmts, &build.AssignExpr{
			LHS: bazelIdent("go_deps_dev"),
			Op:  "=",
			RHS: bazelCall("use_extension",
				stringExpr("@gazelle//:extensions.bzl"),
				stringExpr("go_deps"),
				bazelAssign("dev_dependency", bazelIdent("True")),
			),
		})
		devTagStmts, err := goDepsTagStmts("go_deps_dev", *m.TagsDev, m.Name)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, devTagStmts...)
	}

	if tagsNonempty(m.TagsIsolate) {
		stmts = append(stmts, &build.AssignExpr{
			LHS: bazelIdent("go_deps_isolate"),
			Op:  "=",
			RHS: bazelCall("use_extension",
				stringExpr("@gazelle//:extensions.bzl"),
				stringExpr("go_deps"),
				bazelAssign("isolate", bazelIdent("True")),
			),
		})
		isolateTagStmts, err := goDepsTagStmts("go_deps_isolate", *m.TagsIsolate, m.Name)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, isolateTagStmts...)
	}

	if m.IsRoot {
		stmts = append(stmts, bazelCall("use_repo",
			bazelIdent("go_deps"),
			stringExpr("bazel_gazelle_go_repository_config"),
		))
	}

	return &build.File{
		Path: "MODULE.bazel",
		Type: build.TypeModule,
		Stmt: stmts,
	}, nil
}

func goDepsTagStmts(goDepsVar string, t tags, moduleName string) ([]build.Expr, error) {
	var stmts []build.Expr
	emit := func(tagType string, items []map[string]any, fieldOrder []string) error {
		for _, item := range items {
			args, err := tagFieldAssigns(item, fieldOrder, moduleName)
			if err != nil {
				return err
			}
			stmts = append(stmts, bazelDotCall(goDepsVar, tagType, args...))
		}
		return nil
	}

	if err := emit("config", t.Config, configFieldOrder); err != nil {
		return nil, err
	}
	if err := emit("from_file", t.FromFile, fromFileFieldOrder); err != nil {
		return nil, err
	}
	if err := emit("gazelle_default_attributes", t.GazelleDefaultAttributes, gazelleDefaultAttributesFieldOrder); err != nil {
		return nil, err
	}
	if err := emit("module", t.Module, moduleTagFieldOrder); err != nil {
		return nil, err
	}
	if err := emit("gazelle_override", t.GazelleOverride, gazelleOverrideFieldOrder); err != nil {
		return nil, err
	}
	if err := emit("module_override", t.ModuleOverride, moduleOverrideFieldOrder); err != nil {
		return nil, err
	}
	if err := emit("archive_override", t.ArchiveOverride, archiveOverrideFieldOrder); err != nil {
		return nil, err
	}
	return stmts, nil
}

var (
	configFieldOrder                   = []string{"check_direct_dependencies", "go_env", "go_env_inherit", "debug_mode"}
	fromFileFieldOrder                 = []string{"go_mod", "go_work", "fail_on_version_conflict"}
	gazelleDefaultAttributesFieldOrder = []string{"build_file_generation", "build_extra_args", "directives"}
	moduleTagFieldOrder                = []string{"path", "sum", "version", "indirect", "build_naming_convention", "build_file_proto_mode", "local_path"}
	gazelleOverrideFieldOrder          = []string{"path", "build_file_generation", "build_extra_args", "directives"}
	moduleOverrideFieldOrder           = []string{"path", "patches", "patch_strip", "patch_cmds", "repo_name"}
	archiveOverrideFieldOrder          = []string{"path", "urls", "strip_prefix", "sha256", "patches", "patch_strip", "patch_cmds"}
)

func tagFieldAssigns(item map[string]any, fieldOrder []string, moduleName string) ([]build.Expr, error) {
	written := map[string]bool{}
	var assigns []build.Expr
	for _, key := range fieldOrder {
		value, ok := item[key]
		if !ok {
			continue
		}
		expr, err := tagFieldExpr(key, value, moduleName)
		if err != nil {
			return nil, err
		}
		assigns = append(assigns, bazelAssign(key, expr))
		written[key] = true
	}
	keys := make([]string, 0, len(item))
	for key := range item {
		if !written[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		expr, err := tagFieldExpr(key, item[key], moduleName)
		if err != nil {
			return nil, err
		}
		assigns = append(assigns, bazelAssign(key, expr))
	}
	return assigns, nil
}

func tagFieldExpr(key string, value any, moduleName string) (build.Expr, error) {
	if key == "go_mod" || key == "go_work" {
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected string", key)
		}
		return stringExpr(labelForModule(s, moduleName)), nil
	}
	return valueToExpr(value)
}

func labelForModule(label, moduleName string) string {
	if m := labelRE.FindStringSubmatch(label); m != nil {
		if m[1] == moduleName {
			return "//" + m[2]
		}
	}
	return label
}

func valueToExpr(value any) (build.Expr, error) {
	switch v := value.(type) {
	case nil:
		return bazelIdent("None"), nil
	case bool:
		if v {
			return bazelIdent("True"), nil
		}
		return bazelIdent("False"), nil
	case float64:
		if v == float64(int64(v)) {
			return &build.LiteralExpr{Token: strconv.FormatInt(int64(v), 10)}, nil
		}
		return &build.LiteralExpr{Token: strconv.FormatFloat(v, 'g', -1, 64)}, nil
	case string:
		return stringExpr(v), nil
	case []any:
		return listToExpr(v)
	case map[string]any:
		return dictToExpr(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return &build.LiteralExpr{Token: strconv.FormatInt(i, 10)}, nil
		}
		return &build.LiteralExpr{Token: v.String()}, nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", value)
	}
}

func listToExpr(items []any) (build.Expr, error) {
	if len(items) == 0 {
		return &build.ListExpr{}, nil
	}
	list := make([]build.Expr, 0, len(items))
	for _, item := range items {
		expr, err := valueToExpr(item)
		if err != nil {
			return nil, err
		}
		list = append(list, expr)
	}
	return &build.ListExpr{List: list, ForceMultiLine: true}, nil
}

func dictToExpr(items map[string]any) (build.Expr, error) {
	if len(items) == 0 {
		return &build.DictExpr{}, nil
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]*build.KeyValueExpr, 0, len(keys))
	for _, key := range keys {
		value, err := valueToExpr(items[key])
		if err != nil {
			return nil, err
		}
		entries = append(entries, &build.KeyValueExpr{
			Key:   stringExpr(key),
			Value: value,
		})
	}
	return &build.DictExpr{List: entries, ForceMultiLine: true}, nil
}

func stringExpr(s string) *build.StringExpr {
	return &build.StringExpr{Value: s}
}

func bazelIdent(name string) *build.Ident {
	return &build.Ident{Name: name}
}

func bazelAssign(name string, value build.Expr) *build.AssignExpr {
	return &build.AssignExpr{
		LHS: bazelIdent(name),
		Op:  "=",
		RHS: value,
	}
}

func bazelCall(name string, args ...build.Expr) *build.CallExpr {
	return &build.CallExpr{
		X:    bazelIdent(name),
		List: args,
	}
}

func bazelDotCall(varName, method string, args ...build.Expr) *build.CallExpr {
	return &build.CallExpr{
		X: &build.DotExpr{
			X:    bazelIdent(varName),
			Name: method,
		},
		List: args,
	}
}

const goDepsGoVersion = "1.24.12"

var bazelLabelRE = regexp.MustCompile(`^@+([^/]+)//([^:]*):([^:]*)$`)

type goModRef struct {
	bazelModule string
	isRoot      bool
	label       string
	fileKey     string
}

type replaceDirective struct {
	oldPath string
	oldVers string
	newPath string
	newVers string
}

type goDepsWorkspace struct {
	usePaths        []string
	replaces        []replaceDirective
	goWorkSum       []string
	moduleTags      []map[string]any
	bazelGoModDirs  map[string]string // Go module path => directory in synthetic workspace
}

func writeGoDepsWorkFiles(dirPath string, tc *testCase) error {
	if err := writeGoDepsWorkDir(dirPath, tc, "", false); err != nil {
		return err
	}
	for i := range tc.Modules {
		m := &tc.Modules[i]
		if tagsNonempty(m.TagsIsolate) {
			if err := writeGoDepsWorkDir(dirPath, tc, m.Name, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeGoDepsWorkDir(dirPath string, tc *testCase, isolateModuleName string, isolated bool) error {
	ws, err := buildGoDepsWorkspace(dirPath, tc, isolateModuleName, isolated)
	if err != nil {
		return err
	}
	if ws == nil {
		return nil
	}

	workDir := filepath.Join(dirPath, goDepsWorkDir(isolateModuleName, isolated))
	if err := os.MkdirAll(workDir, 0777); err != nil {
		return err
	}

	files := map[string]string{
		"go.mod":  renderGoDepsGoMod(ws),
		"go.sum":  renderGoDepsGoSum(ws),
		"go.work": renderGoDepsGoWork(ws),
	}
	if len(ws.goWorkSum) > 0 {
		files["go.work.sum"] = strings.Join(ws.goWorkSum, "\n") + "\n"
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(workDir, name), []byte(content), 0666); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func buildGoDepsWorkspace(dirPath string, tc *testCase, isolateModuleName string, isolated bool) (*goDepsWorkspace, error) {
	ws := &goDepsWorkspace{
		moduleTags: collectModuleTagsForWorkspace(tc, isolateModuleName, isolated),
	}
	if isolated && len(ws.moduleTags) == 0 && !hasFromFileTags(tc, isolateModuleName, isolated) {
		return nil, nil
	}

	seenGoMod := map[string]bool{}
	seenUse := map[string]bool{}

	addUse := func(path string) {
		path = filepath.ToSlash(path)
		if path == "." || path == "" || seenUse[path] {
			return
		}
		seenUse[path] = true
		ws.usePaths = append(ws.usePaths, path)
	}

	visitGoMod := func(ref goModRef) error {
		if seenGoMod[ref.fileKey] {
			return nil
		}
		seenGoMod[ref.fileKey] = true

		content, ok := tc.Files[ref.fileKey]
		if !ok {
			return fmt.Errorf("missing go.mod content for label %q (expected files[%q])", ref.label, ref.fileKey)
		}

		origGoModDir := filepath.Join(dirPath, filepath.Dir(strings.TrimPrefix(ref.fileKey, "./")))
		actsAsRoot := ref.isRoot || (isolated && ref.bazelModule == isolateModuleName)
		transformed, err := transformGoMod([]byte(content), actsAsRoot, origGoModDir)
		if err != nil {
			return fmt.Errorf("transform %s: %w", ref.fileKey, err)
		}

		copyPath := labelToModCopyPath(ref.label)
		fullCopyPath := filepath.Join(dirPath, goDepsWorkDir(isolateModuleName, isolated), copyPath)
		if err := os.MkdirAll(filepath.Dir(fullCopyPath), 0777); err != nil {
			return err
		}
		if err := os.WriteFile(fullCopyPath, transformed, 0666); err != nil {
			return fmt.Errorf("write %s: %w", copyPath, err)
		}

		sumKey := strings.TrimSuffix(ref.fileKey, "go.mod") + "go.sum"
		if sumContent, ok := tc.Files[sumKey]; ok {
			sumCopyPath := filepath.Join(filepath.Dir(fullCopyPath), "go.sum")
			if err := os.WriteFile(sumCopyPath, []byte(sumContent), 0666); err != nil {
				return fmt.Errorf("write %s: %w", sumCopyPath, err)
			}
		}

		usePath := labelToModUsePath(ref.label)
		addUse(usePath)

		mf, err := modfile.Parse("go.mod", []byte(content), nil)
		if err != nil {
			return fmt.Errorf("parse %s: %w", ref.fileKey, err)
		}
		if mf.Module != nil && mf.Module.Mod.Path != "" {
			if ws.bazelGoModDirs == nil {
				ws.bazelGoModDirs = map[string]string{}
			}
			ws.bazelGoModDirs[mf.Module.Mod.Path] = usePath
		}
		return nil
	}

	for i := range tc.Modules {
		m := &tc.Modules[i]
		if isolated {
			if m.Name != isolateModuleName || m.TagsIsolate == nil {
				continue
			}
			for _, tag := range m.TagsIsolate.FromFile {
				if err := processFromFileTag(dirPath, tc, m, tag, visitGoMod, addUse, ws, isolated, isolateModuleName); err != nil {
					return nil, err
				}
			}
			continue
		}
		tagSets := []*tags{m.Tags, m.TagsDev}
		for _, tagSet := range tagSets {
			if tagSet == nil {
				continue
			}
			for _, tag := range tagSet.FromFile {
				if err := processFromFileTag(dirPath, tc, m, tag, visitGoMod, addUse, ws, isolated, isolateModuleName); err != nil {
					return nil, err
				}
			}
		}
	}
	return ws, nil
}

func hasFromFileTags(tc *testCase, isolateModuleName string, isolated bool) bool {
	for i := range tc.Modules {
		m := &tc.Modules[i]
		if isolated {
			if m.Name == isolateModuleName && m.TagsIsolate != nil && len(m.TagsIsolate.FromFile) > 0 {
				return true
			}
			continue
		}
		if m.Tags != nil && len(m.Tags.FromFile) > 0 {
			return true
		}
		if m.TagsDev != nil && len(m.TagsDev.FromFile) > 0 {
			return true
		}
	}
	return false
}

func collectModuleTagsForWorkspace(tc *testCase, isolateModuleName string, isolated bool) []map[string]any {
	if isolated {
		for i := range tc.Modules {
			m := &tc.Modules[i]
			if m.Name == isolateModuleName && m.TagsIsolate != nil {
				return m.TagsIsolate.Module
			}
		}
		return nil
	}
	return collectModuleTags(tc)
}

func processFromFileTag(dirPath string, tc *testCase, m *module, tag map[string]any, visitGoMod func(goModRef) error, addUse func(string), ws *goDepsWorkspace, isolated bool, isolateModuleName string) error {
	goMod, hasGoMod := tag["go_mod"].(string)
	goWork, hasGoWork := tag["go_work"].(string)
	switch {
	case hasGoMod && goMod != "":
		ref, err := goModRefFromLabel(goMod, m.Name, m.IsRoot)
		if err != nil {
			return err
		}
		return visitGoMod(ref)
	case hasGoWork && goWork != "":
		return processGoWorkFromFileTag(dirPath, tc, m, goWork, visitGoMod, addUse, ws, isolated, isolateModuleName)
	}
	return nil
}

func goModRefFromLabel(label, bazelModule string, isRoot bool) (goModRef, error) {
	fileKey, err := labelToFileKey(label)
	if err != nil {
		return goModRef{}, err
	}
	return goModRef{
		bazelModule: bazelModule,
		isRoot:      isRoot,
		label:       label,
		fileKey:     fileKey,
	}, nil
}

func processGoWorkFromFileTag(dirPath string, tc *testCase, m *module, goWorkLabel string, visitGoMod func(goModRef) error, addUse func(string), ws *goDepsWorkspace, isolated bool, isolateModuleName string) error {
	fileKey, err := labelToFileKey(goWorkLabel)
	if err != nil {
		return err
	}
	content, ok := tc.Files[fileKey]
	if !ok {
		return fmt.Errorf("missing go.work content for label %q (expected files[%q])", goWorkLabel, fileKey)
	}

	wf, err := modfile.ParseWork("go.work", []byte(content), nil)
	if err != nil {
		return fmt.Errorf("parse %s: %w", fileKey, err)
	}

	for _, u := range wf.Use {
		if isRelativeUsePath(u.Path) {
			goModLabel, err := goModLabelFromGoWork(goWorkLabel, u.Path)
			if err != nil {
				return err
			}
			ref, err := goModRefFromLabel(goModLabel, m.Name, m.IsRoot)
			if err != nil {
				return err
			}
			if err := visitGoMod(ref); err != nil {
				return err
			}
		} else {
			addUse(u.Path)
		}
	}

	actsAsRoot := m.IsRoot || (isolated && m.Name == isolateModuleName)
	if actsAsRoot {
		absGoWorkDir := filepath.Join(dirPath, filepath.Dir(strings.TrimPrefix(fileKey, "./")))
		for _, r := range wf.Replace {
			newPath := r.New.Path
			newVers := r.New.Version
			if newVers == "" && isRelativeReplacePath(newPath) {
				newPath = filepath.ToSlash(filepath.Clean(filepath.Join(absGoWorkDir, newPath)))
			}
			ws.replaces = append(ws.replaces, replaceDirective{
				oldPath: r.Old.Path,
				oldVers: r.Old.Version,
				newPath: newPath,
				newVers: newVers,
			})
		}
	}

	sumKey := strings.TrimSuffix(fileKey, "go.work") + "go.work.sum"
	if sumContent, ok := tc.Files[sumKey]; ok {
		ws.goWorkSum = append(ws.goWorkSum, strings.TrimSpace(sumContent))
	}
	return nil
}

func transformGoMod(content []byte, isRoot bool, absGoModDir string) ([]byte, error) {
	mf, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return nil, err
	}
	if !isRoot {
		for _, r := range append([]*modfile.Replace(nil), mf.Replace...) {
			if err := mf.DropReplace(r.Old.Path, r.Old.Version); err != nil {
				return nil, err
			}
		}
		for _, e := range append([]*modfile.Exclude(nil), mf.Exclude...) {
			if err := mf.DropExclude(e.Mod.Path, e.Mod.Version); err != nil {
				return nil, err
			}
		}
	} else if err := fixReplacePaths(mf, absGoModDir); err != nil {
		return nil, err
	}
	mf.Cleanup()
	return mf.Format()
}

func fixReplacePaths(mf *modfile.File, absGoModDir string) error {
	for _, r := range append([]*modfile.Replace(nil), mf.Replace...) {
		if r.New.Version == "" && isRelativeReplacePath(r.New.Path) {
			newPath := filepath.ToSlash(filepath.Clean(filepath.Join(absGoModDir, r.New.Path)))
			if err := mf.AddReplace(r.Old.Path, r.Old.Version, newPath, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

func isRelativeUsePath(path string) bool {
	return path == "." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func isRelativeReplacePath(path string) bool {
	return path == "." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func goModLabelFromGoWork(goWorkLabel, useDiskPath string) (string, error) {
	m := bazelLabelRE.FindStringSubmatch(goWorkLabel)
	if m == nil {
		return "", fmt.Errorf("invalid go.work label %q", goWorkLabel)
	}
	modName, pkg := m[1], m[2]
	joined := filepath.ToSlash(filepath.Clean(filepath.Join(pkg, useDiskPath)))
	if joined == "." {
		joined = ""
	}
	if joined == "" {
		return fmt.Sprintf("@@%s//:go.mod", modName), nil
	}
	return fmt.Sprintf("@@%s//%s:go.mod", modName, joined), nil
}

func labelToModCopyPath(label string) string {
	m := bazelLabelRE.FindStringSubmatch(label)
	if m == nil {
		return ""
	}
	modName, pkg, name := m[1], m[2], m[3]
	rel := name
	if pkg != "" {
		rel = pkg + "/" + name
	}
	return filepath.ToSlash(filepath.Join("mod", modName, rel))
}

func labelToModUsePath(label string) string {
	return filepath.ToSlash(filepath.Dir(labelToModCopyPath(label)))
}

func localReplacePath(path string) string {
	path = filepath.ToSlash(path)
	if path == "." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || filepath.IsAbs(path) {
		return path
	}
	return "./" + path
}

func renderGoDepsGoMod(ws *goDepsWorkspace) string {
	var b strings.Builder
	b.WriteString("\nmodule go_deps_module_tags\n\ngo ")
	b.WriteString(goDepsGoVersion)
	b.WriteString("\n\n")
	for _, tag := range ws.moduleTags {
		path, _ := tag["path"].(string)
		version, _ := tag["version"].(string)
		fmt.Fprintf(&b, "require %s %s\n", path, version)
		localPath, _ := tag["local_path"].(string)
		if localPath == "" {
			if dir, ok := ws.bazelGoModDirs[path]; ok {
				fmt.Fprintf(&b, "replace %s %s => %s\n", path, version, localReplacePath(dir))
			}
		}
	}
	return b.String()
}

func renderGoDepsGoSum(ws *goDepsWorkspace) string {
	var b strings.Builder
	for _, tag := range ws.moduleTags {
		sum, _ := tag["sum"].(string)
		if sum == "" {
			continue
		}
		fmt.Fprintf(&b, "%s %s %s\n", tag["path"], tag["version"], sum)
	}
	return b.String()
}

func renderGoDepsGoWork(ws *goDepsWorkspace) string {
	var b strings.Builder
	b.WriteString("\ngo ")
	b.WriteString(goDepsGoVersion)
	b.WriteString("\n\nuse .\n")
	for _, usePath := range ws.usePaths {
		b.WriteString("use ")
		b.WriteString(usePath)
		b.WriteString("\n")
	}
	for _, r := range ws.replaces {
		b.WriteString("replace ")
		b.WriteString(r.oldPath)
		if r.oldVers != "" {
			b.WriteString(" ")
			b.WriteString(r.oldVers)
		}
		b.WriteString(" => ")
		b.WriteString(r.newPath)
		if r.newVers != "" {
			b.WriteString(" ")
			b.WriteString(r.newVers)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func collectModuleTags(tc *testCase) []map[string]any {
	var tags []map[string]any
	for i := range tc.Modules {
		m := &tc.Modules[i]
		if m.Tags != nil {
			tags = append(tags, m.Tags.Module...)
		}
		if m.TagsDev != nil {
			tags = append(tags, m.TagsDev.Module...)
		}
	}
	return tags
}

func labelToFileKey(label string) (string, error) {
	m := bazelLabelRE.FindStringSubmatch(label)
	if m == nil {
		return "", fmt.Errorf("invalid label %q", label)
	}
	modName, pkg, name := m[1], m[2], m[3]
	if pkg == "" {
		return "./" + modName + "/" + name, nil
	}
	return "./" + modName + "/" + pkg + "/" + name, nil
}
