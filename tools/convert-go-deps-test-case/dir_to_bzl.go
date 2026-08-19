package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

type parsedModule struct {
	name        string
	version     string
	tags        tags
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
		modules = append(modules, m)
	}

	tc := testCase{
		Name:    testName,
		Modules: modules,
		Files:   files,
		Want:    want,
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
	goDepsVar := "go_deps"

	for _, stmt := range file.Stmt {
		call, ok := stmt.(*build.CallExpr)
		if !ok {
			if assign, ok := stmt.(*build.AssignExpr); ok {
				if ident, ok := assign.LHS.(*build.Ident); ok {
					if rhsCall, ok := assign.RHS.(*build.CallExpr); ok && callName(rhsCall) == "use_extension" {
						goDepsVar = ident.Name
					}
				}
			}
			continue
		}

		if tagType, ok := goDepsTagType(call, goDepsVar); ok {
			attrs, err := callAttrs(call)
			if err != nil {
				return nil, err
			}
			attrs = labelAttrsForTestCase(pm.name, attrs)
			attrs = omitTagDefaults(tagType, attrs)
			if err := appendTag(pm, tagType, attrs); err != nil {
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

func goDepsTagType(call *build.CallExpr, goDepsVar string) (string, bool) {
	dot, ok := call.X.(*build.DotExpr)
	if !ok {
		return "", false
	}
	ident, ok := dot.X.(*build.Ident)
	if !ok || ident.Name != goDepsVar {
		return "", false
	}
	return dot.Name, true
}

func callName(call *build.CallExpr) string {
	switch fn := call.X.(type) {
	case *build.Ident:
		return fn.Name
	case *build.DotExpr:
		return fn.Name
	default:
		return ""
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

func appendTag(pm *parsedModule, tagType string, attrs map[string]any) error {
	if len(attrs) == 0 {
		attrs = map[string]any{}
	}
	switch tagType {
	case "archive_override":
		pm.tags.ArchiveOverride = append(pm.tags.ArchiveOverride, attrs)
	case "config":
		pm.tags.Config = append(pm.tags.Config, attrs)
	case "from_file":
		pm.tags.FromFile = append(pm.tags.FromFile, attrs)
	case "gazelle_override":
		pm.tags.GazelleOverride = append(pm.tags.GazelleOverride, attrs)
	case "gazelle_default_attributes":
		pm.tags.GazelleDefaultAttributes = append(pm.tags.GazelleDefaultAttributes, attrs)
	case "module":
		pm.tags.Module = append(pm.tags.Module, attrs)
	case "module_override":
		pm.tags.ModuleOverride = append(pm.tags.ModuleOverride, attrs)
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

func readWantJSON(dirPath string) (json.RawMessage, error) {
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
	return json.RawMessage(data), nil
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
