// Copyright 2024 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package convention

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const autoResolvesHeader = "### AUTOMATIC RESOLVES ###"

// Convention should be implemented by langauge extensions in order to
// register language specific convention logic with the convention checker.
type Convention interface {
	// CheckConvention returns whether or not the rule information follows
	// a known convention.
	CheckConvention(c *config.Config, kind, imp, name, rel string) bool
}

// resolveSpec contains the rule information used by Finish() to create
// new resolve directives.
// rule and file are the rule's underlying syntax information.
type resolveSpec struct {
	imps  []resolve.ImportSpec
	label label.Label
	rule  *rule.Rule
	file  *rule.File
}

// metaResolver returns the Resolver associated with the given
// rule and package, or nil if it doesn't have one.
type metaResolver func(r *rule.Rule, pkgRel string) resolve.Resolver

// Checker checks to see if rules follow a convention, and if not writes
// them as # resolve directive to the top level BUILD.bazel.
type Checker struct {
	dirsRel      []string
	metaResolver metaResolver
	resolves     []resolveSpec
	conventions  []Convention
}

// NewChecker creates a new Checker object.
// dirs is the list of passed in dirs for this Gazelle run.
// metaresolver is used for getting a rule's imports.
// exts is used for finding the conventions for a given rule.
func NewChecker(c *config.Config, dirs []string, metaResolver func(r *rule.Rule, pkgRel string) resolve.Resolver, exts ...interface{}) *Checker {
	if !isEnabled(c) {
		return &Checker{}
	}
	var conventions []Convention
	for _, e := range exts {
		if c, ok := e.(Convention); ok {
			conventions = append(conventions, c)
		}
	}
	checker := &Checker{
		metaResolver: metaResolver,
		conventions:  conventions,
	}
	checker.dirsRel = make([]string, len(dirs))
	for i, d := range dirs {
		if d == c.RepoRoot {
			checker.dirsRel[i] = ""
		} else {
			checker.dirsRel[i] = strings.TrimPrefix(d, c.RepoRoot+string(filepath.Separator))
		}
	}
	return checker
}

func isEnabled(c *config.Config) bool {
	cc := getConventionConfig(c)
	return cc != nil && cc.genResolves
}

// AddRule checks all of supplied rule's imports against the rule's corresponding convention.
// Rules that don't match convention are saved, so they can be written as top-level # resolve
// directives once Finish() is called.
func (ch *Checker) AddRule(c *config.Config, r *rule.Rule, f *rule.File) {
	if !isEnabled(c) {
		return
	}
	var imps []resolve.ImportSpec
	if rslv := ch.metaResolver(r, f.Pkg); rslv != nil {
		imps = rslv.Imports(c, r, f)
	}
	unconventionalImps := imps[:0]
	for _, imp := range imps {
		var conventional bool
		for _, conv := range ch.conventions {
			if conv.CheckConvention(c, r.Kind(), imp.Imp, r.Name(), f.Pkg) {
				conventional = true
				break
			}
		}
		if !conventional {
			unconventionalImps = append(unconventionalImps, imp)
		}
	}
	if len(unconventionalImps) > 0 {
		ch.resolves = append(ch.resolves, resolveSpec{
			imps:  unconventionalImps,
			label: label.New("", f.Pkg, r.Name()),
			rule:  r,
			file:  f,
		})
	}
}

type directive struct {
	imp   resolve.ImportSpec
	label label.Label
}

// expected format: # gazelle:resolve source-language import-language import-string label
// for example: # gazelle:resolve go go example.com/foo //src/foo:go_default_library
func parseDirective(line string) (d directive, err error) {
	parts := strings.Fields(strings.TrimPrefix(line, "# gazelle:resolve "))
	var imp resolve.ImportSpec
	switch len(parts) {
	case 3:
		imp.Lang = parts[0]
		imp.Imp = parts[1]
	case 4:
		imp.Lang = parts[1]
		imp.Imp = parts[2]
	default:
		return d, fmt.Errorf("could not parse directive: %q\n\texpected # gazelle:resolve source-language [import-language] import-string label", line)
	}
	label, err := label.Parse(parts[len(parts)-1])
	if err != nil {
		return d, fmt.Errorf("invalid label %q: %v", line, err)
	}
	return directive{
		label: label,
		imp: resolve.ImportSpec{
			Lang: parts[1],
			Imp:  parts[2],
		},
	}, err
}

// getHeaderPosition scans the file and returns the position of the autoResolvesHeader
func getHeaderPosition(r io.Reader) (int64, error) {
	scanner := bufio.NewScanner(r)
	var startPos int64
	for scanner.Scan() {
		line := scanner.Text()
		startPos += int64(len(line)) + int64(1) // +1 for new line
		if line == autoResolvesHeader {
			return startPos, scanner.Err()
		}
	}
	return startPos, fmt.Errorf("Failure finding header: %s in top-level BUILD.bazel", autoResolvesHeader)
}

// readDirectives seeks the file to startPos and then reads in all of the
// # gazelle:resolve directives into an import => directive map
func readDirectives(r *os.File, startPos int64) (map[string]directive, error) {
	if _, err := r.Seek(startPos, 0); err != nil {
		return nil, err
	}
	directivesMap := make(map[string]directive) // import => directive
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		d, err := parseDirective(line)
		if err != nil {
			return directivesMap, err
		}
		directivesMap[d.imp.Imp] = d
	}
	return directivesMap, scanner.Err()
}

func (ch *Checker) finish(c *config.Config, index *resolve.RuleIndex) error {
	if !isEnabled(c) {
		return nil
	}
	f, err := os.OpenFile(path.Join(c.RepoRoot, "BUILD.bazel"), os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	headerPos, err := getHeaderPosition(f)
	if err != nil {
		return err
	}
	directivesMap, err := readDirectives(f, headerPos) // import => directive
	if err != nil {
		return err
	}

	hasNewResolve := false
	// Collect resolve directives from newly generated rules during current Gazelle run.
	// Because Gazelle only updates rules in ch.dirsRel, these new resolves all point to those directories.
	newDirectives := make(map[string]directive)
	for _, spec := range ch.resolves {
		hasNew := false
		for _, imp := range spec.imps {
			newDirectives[imp.Imp] = directive{
				label: spec.label,
				imp:   imp,
			}
			if d, ok := directivesMap[imp.Imp]; !ok || !d.label.Equal(spec.label) || d.imp.Lang != imp.Lang {
				hasNew = true
			}
		}
		if hasNew {
			hasNewResolve = true
			// enable indexing and add the unconventional rule to the index
			// that way they can be resolved by the current Gazelle run
			c.IndexLibraries = true
			index.AddRule(c, spec.rule, spec.file)
		}
	}

	cc := getConventionConfig(c)
	givenDirs := NewDirSet(ch.dirsRel)
	directives, hasOutdatedResolve := replaceDirectivesInScope(directivesMap, newDirectives, func(dir string) bool {
		return cc.recursiveMode && givenDirs.HasSubDir(dir) || !cc.recursiveMode && givenDirs.hasDir(dir)
	})
	if !hasOutdatedResolve && !hasNewResolve {
		// no new directives and no old directives, don't need to rewrite
		return nil
	}

	// write updated directive list back to BUILD.bazel beginning at headerPos
	if err := f.Truncate(headerPos); err != nil {
		return err
	}
	if _, err := f.Seek(headerPos, 0); err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, d := range directives {
		if _, err := fmt.Fprintf(w, "# gazelle:resolve %s %s %s %s\n", d.imp.Lang, d.imp.Lang, d.imp.Imp, d.label); err != nil {
			return err
		}
	}
	return w.Flush()
}

// Finish reads all of the specs collected by AddRule, and for any that do not already have
// a # resolve directive, they are added to the sorted list within the top-level BUILD.bazel.
// New specs are also added to the RuleIndex, that way they can be resolved by the
// current Gazelle run.
func (ch *Checker) Finish(c *config.Config, index *resolve.RuleIndex) {
	if err := ch.finish(c, index); err != nil {
		log.Println(err)
	}
}

func replaceDirectivesInScope(existingDirectives, newDirectives map[string]directive, isInScope func(string) bool) ([]directive, bool) {
	reducedMap := make(map[string]directive, len(existingDirectives))
	for imp, d := range existingDirectives {
		if isInScope(d.label.Pkg) {
			if newDirective, ok := newDirectives[imp]; !ok || !d.label.Equal(newDirective.label) || d.imp.Lang != newDirective.imp.Lang {
				// either the directive is no longer needed, or it resolves to a different language or label.
				continue
			}
		}
		reducedMap[imp] = d
	}
	hasOutdatedResolve := len(reducedMap) < len(existingDirectives)
	// adding new directives, and possibly updating outdated directives that map an import to a different language or label.
	for imp, d := range newDirectives {
		reducedMap[imp] = d
	}

	// sort directives, first by lang and then by import, making it ready to be written back to disk.
	directives := make([]directive, 0, len(reducedMap))
	for _, d := range reducedMap {
		directives = append(directives, d)
	}
	sort.Slice(directives, func(i, j int) bool {
		if directives[i].imp.Lang != directives[j].imp.Lang {
			return directives[i].imp.Lang < directives[j].imp.Lang
		}
		return directives[i].imp.Imp < directives[j].imp.Imp
	})
	return directives, hasOutdatedResolve
}
