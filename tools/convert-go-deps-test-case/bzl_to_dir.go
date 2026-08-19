package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bazelbuild/buildtools/build"
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
		modTags := tags{}
		if m.Tags != nil {
			modTags = *m.Tags
		}
		file, err := buildModuleBazelFile(m, deps, repoRoot, modTags)
		if err != nil {
			return fmt.Errorf("render MODULE.bazel for %s: %w", m.Name, err)
		}
		modDir := filepath.Join(dirPath, m.Name)
		if err := os.MkdirAll(modDir, 0777); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(modDir, "MODULE.bazel"), build.Format(file), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(modDir, "BUILD"), nil, 0o644); err != nil {
			return err
		}
	}

	return nil
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

func writeWantJSON(dirPath string, want json.RawMessage) error {
	if len(want) == 0 || string(want) == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal(want, &value); err != nil {
		return fmt.Errorf("decode want field: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode want.json: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dirPath, "want.json"), data, 0666)
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

func buildModuleBazelFile(m *module, deps []module, gazelleRoot string, modTags tags) (*build.File, error) {
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

	tagStmts, err := goDepsTagStmts("go_deps", modTags, m.Name)
	if err != nil {
		return nil, err
	}
	stmts = append(stmts, tagStmts...)

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
