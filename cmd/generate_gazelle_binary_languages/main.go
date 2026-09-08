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

// generate_gazelle_binary_languages is a tool invoked by gazelle_binary
// to generate a .go file containing a list of language extensions.
// The generated list is compatible with either cmd/gazelle or
// v2/cmd/gazelle. It uses the v2 extension type; v1 extensions use
// a compatibility adapter.
//
// -o must be set to the output file path.
//
// Positional arguments are compiled Go export data files or archives
// containing export data. A package may have either a v1 or v2
// signature:
//
//	// v1
//	import "github.com/bazelbuild/bazel-gazelle/language"
//	func NewLanguage() language.Language
//
//	// v2
//	import "github.com/bazel-contrib/bazel-gazelle/v2/language"
//	func NewV2() language.Language
//
// When both functions are provided, v2 is preferred.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"slices"
	"text/template"

	"golang.org/x/tools/go/gcexportdata"
)

const (
	v1LanguagePkg = "github.com/bazelbuild/bazel-gazelle/language"
	v2LanguagePkg = "github.com/bazel-contrib/bazel-gazelle/v2/language"
	v2CompatPkg   = "github.com/bazel-contrib/bazel-gazelle/v2/compat"
)

type extension struct {
	ImportPath string
	PkgName    string
	LocalName  string
	FuncName   string
	CompatWrap bool // true for v1 extensions
}

func main() {
	if wd := os.Getenv("BUILD_WORKING_DIRECTORY"); wd != "" {
		if err := os.Chdir(wd); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("generate_gazelle_binary_languages", flag.ContinueOnError)
	var outPath string
	flags.StringVar(&outPath, "o", "", "output .go file name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if outPath == "" {
		return fmt.Errorf("-o must be set")
	}
	extPaths := flags.Args()
	if len(extPaths) == 0 {
		return fmt.Errorf("arguments expected: a list of compiled Go export data files or archives")
	}

	pkgNameCounts := map[string]int{
		"language": 1,
		"compat":   1,
	}
	extensions := make([]extension, 0, len(extPaths))
	var errs []error
	for _, extPath := range extPaths {
		pkg, err := parseExportData(extPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ext, err := makeExtensionFromExportData(pkg, pkgNameCounts)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		extensions = append(extensions, ext)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	content := generateOutput(extensions)
	return os.WriteFile(outPath, content, 0o666)
}

func parseExportData(path string) (*types.Package, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parsing export data: %w", err)
	}
	defer f.Close()

	r, err := gcexportdata.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("parsing export data: %s: %w", path, err)
	}

	pkg, err := gcexportdata.Read(r, token.NewFileSet(), make(map[string]*types.Package), "")
	if err != nil {
		return nil, fmt.Errorf("parsing export data: %s: %w", path, err)
	}
	return pkg, nil
}

func makeExtensionFromExportData(pkg *types.Package, pkgNameCounts map[string]int) (extension, error) {
	pkgPath := pkg.Path()
	ext := extension{
		ImportPath: pkgPath,
		PkgName:    pkg.Name(),
		LocalName:  uniquePackageName(pkg.Name(), pkgNameCounts),
	}

	if fn := findLanguageFunc(pkg, "NewV2", v2LanguagePkg); fn != nil {
		ext.FuncName = "NewV2"
		return ext, nil
	}
	if fn := findLanguageFunc(pkg, "NewLanguage", v1LanguagePkg); fn != nil {
		ext.FuncName = "NewLanguage"
		ext.CompatWrap = true
		return ext, nil
	}
	return ext, fmt.Errorf("package %s: no NewV2() or NewLanguage() function returning language.Language", pkgPath)
}

func uniquePackageName(name string, pkgNameCounts map[string]int) string {
	for {
		pkgNameCounts[name]++
		if pkgNameCounts[name] == 1 {
			return name
		}
		uniqueName := fmt.Sprintf("%s_%d", name, pkgNameCounts[name])
		if pkgNameCounts[uniqueName] == 0 {
			pkgNameCounts[uniqueName] = 1
			return uniqueName
		}
	}
}

func findLanguageFunc(pkg *types.Package, name, langPkgPath string) *types.Func {
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return nil
	}
	fn, ok := obj.(*types.Func)
	if !ok || !isLanguageConstructor(fn, langPkgPath) {
		return nil
	}
	return fn
}

func isLanguageConstructor(fn *types.Func, langPkgPath string) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	named, ok := sig.Results().At(0).Type().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}
	return obj.Name() == "Language" && obj.Pkg().Path() == langPkgPath
}

const langsTemplate = `// Code generated by generate_gazelle_binary_languages; DO NOT EDIT.

package main

import (
{{- if .NeedCompat}}
	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
{{- end}}
	"github.com/bazel-contrib/bazel-gazelle/v2/language"

{{- range .Extensions }}
	{{if ne .LocalName .PkgName}}{{printf "%s %q" .LocalName .ImportPath}}{{else}}{{printf "%q" .ImportPath}}{{end}}
{{- end}}
)

func init() {
	languages = []language.Language{
{{- range .Extensions}}
		{{if .CompatWrap}}compat.LanguageV2({{.LocalName}}.{{.FuncName}}()){{else}}{{.LocalName}}.{{.FuncName}}(){{end}},
{{- end}}
	}
}
`

type outputSpec struct {
	NeedCompat bool
	Extensions []extension
}

type importSpec struct {
	PkgName    string
	LocalName  string
	ImportPath string
}

func generateOutput(extensions []extension) []byte {
	out := outputSpec{
		NeedCompat: slices.ContainsFunc(extensions, func(e extension) bool { return e.CompatWrap }),
		Extensions: extensions,
	}

	root := template.Must(template.New("langs").Parse(langsTemplate))

	buf := &bytes.Buffer{}
	if err := root.Execute(buf, out); err != nil {
		panic(fmt.Sprintf("template execution failed: %v", err))
	}
	return buf.Bytes()
}
