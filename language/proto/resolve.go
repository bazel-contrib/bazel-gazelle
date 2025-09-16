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
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (*protoLang) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	rel := f.Pkg
	srcs := r.AttrStrings("srcs")
	imports := make([]resolve.ImportSpec, 0, len(srcs))

	stripImportPrefix := r.AttrString("strip_import_prefix")
	importPrefix := r.AttrString("import_prefix")

	for _, src := range srcs {
		transformedImport, ok := transformImport(rel, src, stripImportPrefix, importPrefix)
		if !ok {
			continue
		}
		imports = append(imports, resolve.ImportSpec{Lang: "proto", Imp: transformedImport})
	}
	return imports
}

func (*protoLang) Embeds(r *rule.Rule, from label.Label) []label.Label {
	return nil
}

func (*protoLang) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, importsRaw interface{}, from label.Label) {
	if importsRaw == nil {
		// may not be set in tests.
		return
	}
	imports := importsRaw.([]string)
	r.DelAttr("deps")
	depSet := make(map[string]bool)
	for _, imp := range imports {
		l, err := resolveProto(c, ix, r, imp, from)
		if err == errSkipImport {
			continue
		} else if err != nil {
			log.Print(err)
		} else {
			l = l.Rel(from.Repo, from.Pkg)
			depSet[l.String()] = true
		}
	}
	if len(depSet) > 0 {
		deps := make([]string, 0, len(depSet))
		for dep := range depSet {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		r.SetAttr("deps", deps)
	}
}

var (
	errSkipImport = errors.New("std import")
	errNotFound   = errors.New("not found")
)

func resolveProto(c *config.Config, ix *resolve.RuleIndex, r *rule.Rule, imp string, from label.Label) (label.Label, error) {
	pc := GetProtoConfig(c)
	if !strings.HasSuffix(imp, ".proto") {
		return label.NoLabel, fmt.Errorf("can't import non-proto: %q", imp)
	}

	if l, ok := resolve.FindRuleWithOverride(c, resolve.ImportSpec{Imp: imp, Lang: "proto"}, "proto"); ok {
		return l, nil
	}

	if l, ok := knownImports[imp]; ok && pc.Mode.ShouldUseKnownImports() {
		if l.Equal(from) {
			return label.NoLabel, errSkipImport
		} else {
			if workspaceName, isModule := bazelModuleRepos[l.Repo]; isModule {
				apparentRepoName := c.ModuleToApparentName(l.Repo)
				if apparentRepoName == "" {
					// The user doesn't have a bazel_dep for the module containing this known
					// target.
					// TODO: Fail here instead when not using WORKSPACE anymore.
					l.Repo = workspaceName
				} else {
					l.Repo = apparentRepoName
				}
			}
			return l, nil
		}
	}

	if l, err := resolveWithIndex(c, ix, imp, from); err == nil || err == errSkipImport {
		return l, err
	} else if err != errNotFound {
		return label.NoLabel, err
	}

	rel := path.Dir(imp)
	if rel == "." {
		rel = ""
	}
	name := RuleName(rel)
	return label.New("", rel, name), nil
}

func resolveWithIndex(c *config.Config, ix *resolve.RuleIndex, imp string, from label.Label) (label.Label, error) {
	matches := ix.FindRulesByImportWithConfig(c, resolve.ImportSpec{Lang: "proto", Imp: imp}, "proto")
	if len(matches) == 0 {
		return label.NoLabel, errNotFound
	}
	if len(matches) > 1 {
		return label.NoLabel, fmt.Errorf("multiple rules (%s and %s) may be imported with %q from %s", matches[0].Label, matches[1].Label, imp, from)
	}
	if matches[0].IsSelfImport(from) {
		return label.NoLabel, errSkipImport
	}
	return matches[0].Label, nil
}

// CrossResolve provides dependency resolution logic for the go language extension.
func (*protoLang) CrossResolve(c *config.Config, ix *resolve.RuleIndex, imp resolve.ImportSpec, lang string) []resolve.FindResult {
	if lang != "go" {
		return nil
	}
	pc := GetProtoConfig(c)
	if imp.Lang == "proto" && pc.Mode.ShouldUseKnownImports() {
		if l, ok := knownProtoImports[imp.Imp]; ok {
			return []resolve.FindResult{{Label: l}}
		}
	}
	return nil
}

// transformImport transforms an import string for indexing.
//
// libRel is a slash-separated path to the directory containing the target.
//
// protoName is the Bazel package-relative file name (like "foo.proto" or
// "sub/foo.proto"). The full repo-root-relative path is computed by joining
// libRel and protoName.
//
// stripImportPrefix is the value of the target's strip_import_prefix
// attribute. If it's "", this has no effect. If it's a relative path (including
// "."), both libRel and stripImportPrefix are stripped from rel. If it's an
// absolute path, the leading '/' is removed, and only stripImportPrefix is
// removed from protoRel.
//
// importPrefix is the value of the target's import_prefix attribute.
// It's prepended to protoRel after stripImportPrefix is applied.
//
// Both importPrefix and stripImportPrefix must be clean (with path.Clean)
// if they are non-empty.
func transformImport(libRel, protoName, stripImportPrefix, importPrefix string) (string, bool) {
	// Strip the prefix.
	var effectiveStripImportPrefix string
	if path.IsAbs(stripImportPrefix) {
		effectiveStripImportPrefix = stripImportPrefix[len("/"):]
	} else if stripImportPrefix != "" {
		effectiveStripImportPrefix = path.Join(libRel, stripImportPrefix)
	}

	// Build the repo-root-relative path from package and file name
	protoRel := path.Join(libRel, protoName)
	if !pathtools.HasPrefix(protoRel, effectiveStripImportPrefix) {
		return "", false
	}
	cleanRel := pathtools.TrimPrefix(protoRel, effectiveStripImportPrefix)

	// Apply the new prefix.
	cleanRel = path.Join(importPrefix, cleanRel)
	return cleanRel, true
}
