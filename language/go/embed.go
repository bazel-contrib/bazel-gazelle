/* Copyright 2021 The Bazel Authors. All rights reserved.

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
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	radix "github.com/armon/go-radix"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/walk"
	"golang.org/x/mod/module"
)

// embedResolver maps go:embed patterns in source files to lists of files that
// should appear in embedsrcs attributes.
type embedResolver struct {
	// files is a list of embeddable files and directory trees, rooted in the
	// package directory.
	files []*embeddableNode
}

type embeddableNode struct {
	path    string
	entries []*embeddableNode // non-nil for directories
}

func (f *embeddableNode) isDir() bool {
	return f.entries != nil
}

func (f *embeddableNode) isHidden() bool {
	base := path.Base(f.path)
	return strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_")
}

// newEmbedResolver builds a set of files that may be embedded. This is
// approximately all files reachable from a Bazel package directory, including
// explicitly declared generated files and files in subdirectories.
//
// This function walks subdirectory trees and may be expensive. Don't call it
// unless a go:embed directive is actually present.
//
// dir is the absolute path to the directory containing the embed directive.
//
// subdirs, regFiles, and genFiles are lists of subdirectories, regular files,
// and declared generated files in dir, respectively.
func newEmbedResolver(dir string, subdirs, regFiles, genFiles []string) *embedResolver {
	root := &embeddableNode{entries: []*embeddableNode{}}
	index := make(map[string]*embeddableNode)

	var add func(string, bool) *embeddableNode
	add = func(rel string, isDir bool) *embeddableNode {
		if n := index[rel]; n != nil {
			return n
		}
		dir := path.Dir(rel)
		parent := root
		if dir != "." {
			parent = add(dir, true)
		}
		f := &embeddableNode{path: rel}
		if isDir {
			f.entries = []*embeddableNode{}
		}
		parent.entries = append(parent.entries, f)
		index[rel] = f
		return f
	}

	for _, fs := range [...][]string{regFiles, genFiles} {
		for _, f := range fs {
			if !isBadEmbedName(f) {
				add(f, false)
			}
		}
	}

	for _, subdir := range subdirs {
		err := filepath.Walk(filepath.Join(dir, subdir), func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fileRel, _ := filepath.Rel(dir, p)
			fileRel = filepath.ToSlash(fileRel)
			base := filepath.Base(p)
			if !info.IsDir() {
				if !isBadEmbedName(base) {
					add(fileRel, false)
					return nil
				}
				return nil
			}
			if isBadEmbedName(base) {
				return filepath.SkipDir
			}
			add(fileRel, true)
			return nil
		})
		if err != nil {
			log.Printf("listing embeddable files in %s: %v", dir, err)
		}
	}

	return &embedResolver{files: root.entries}
}

// resolve expands a single go:embed pattern into a list of files that should
// be included in embedsrcs. Directory paths are not included in the returned
// list. This means there's no way to embed an empty directory.
func (er *embedResolver) resolve(embed fileEmbed) (list []string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%v: pattern %s: %w", embed.pos, embed.path, err)
		}
	}()

	glob := embed.path
	all := strings.HasPrefix(embed.path, "all:")
	if all {
		glob = strings.TrimPrefix(embed.path, "all:")
	}

	// Check whether the pattern is valid at all.
	if _, err := path.Match(glob, ""); err != nil || !validEmbedPattern(glob) {
		return nil, fmt.Errorf("invalid pattern syntax")
	}

	// Match the pattern against each path in the tree. If the pattern matches a
	// directory, we need to include each file in that directory, even if the file
	// doesn't match the pattern separate. By default, hidden files (starting
	// with . or _) are excluded but all: prefix forces them to be included.
	//
	// For example, the pattern "*" matches "a", ".b", and "_c". If "a" is a
	// directory, we would include "a/d", even though it doesn't match "*". We
	// would not include "a/.e".
	var visit func(*embeddableNode, bool)
	visit = func(f *embeddableNode, add bool) {
		convertedPath := filepath.ToSlash(f.path)
		match, _ := path.Match(glob, convertedPath)
		add = match || (add && (!f.isHidden() || all))
		if !f.isDir() {
			if add {
				list = append(list, convertedPath)
			}
			return
		}
		for _, e := range f.entries {
			visit(e, add)
		}
	}
	for _, f := range er.files {
		visit(f, false)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("matched no files")
	}
	return list, nil
}

// cachedEmbedResolver looks up pre-resolved embed sources for a Go file.
// It maps each Go file (by repo-root-relative path) to the list of embed
// source paths discovered during Configure, and substitutes Bazel labels
// for cross-package sources.
type cachedEmbedResolver struct {
	// resolvedEmbeds stores repo-root-relative file paths resolved from
	// //go:embed directives in ancestor directories as a radix tree.
	// The value is the top most/shallowest directory that embeds it.
	// Populated during Configure (pre-order). In GenerateRules (post-order),
	// child packages remove entries they claim via exports_files.
	// Key: repo-root-relative path (e.g., "embedsrcs/m_go/m.go")
	// Value: rel of the directory that originated the embed
	resolvedEmbeds *radix.Tree
	// relToEmbedSrcs maps a Go file's repo-root-relative path to the
	// repo-root-relative paths of its resolved embed sources.
	relToEmbedSrcs map[string][]string
	// embedSrcLabels maps repo-root-relative embed source paths to Bazel
	// labels for cross-package access.
	embedSrcLabels map[string]label.Label
}

func newCachedEmbedResolver() *cachedEmbedResolver {
	return &cachedEmbedResolver{
		resolvedEmbeds: radix.New(),
		relToEmbedSrcs: make(map[string][]string),
		embedSrcLabels: make(map[string]label.Label),
	}
}

// resolve returns the embed source paths for a Go file identified by its
// repo-root-relative path (fileRel). Cross-package sources are returned as
// Bazel labels; same-package sources are returned as paths relative to the
// file's directory.
func (r *cachedEmbedResolver) resolve(fileRel string) []string {
	srcs := r.relToEmbedSrcs[fileRel]
	if len(srcs) == 0 {
		return nil
	}
	dir := path.Dir(fileRel)
	result := make([]string, 0, len(srcs))
	for _, src := range srcs {
		if l, ok := r.embedSrcLabels[src]; ok {
			result = append(result, l.String())
		} else {
			relToDir := strings.TrimPrefix(src, dir+"/")
			result = append(result, relToDir)
		}
	}
	return result
}

// addEmbedSrc records that a resolved embed source (embedRel, repo-root-relative)
// is referenced by the Go file at fileRel.
// The shallowest originator is preserved in resolvedEmbeds.
func (r *cachedEmbedResolver) addEmbedSrc(fileRel, embedRel string) {
	rel := path.Dir(fileRel)
	// Only record the shallowest originator of an embed source. So that it knows when to stop exporting embedded files.
	// This assumes Configure(), which calles addEmbedSrc eventually, is called in pre-order. 
	// In another words, a parent directory is accessed before its children, thus the first one is the shallowest.
	// A embeded file is exported, if it's not exprted by a sub-package and there's parent package that embeds it.
	if _, found := r.resolvedEmbeds.Get(embedRel); !found {
		r.resolvedEmbeds.Insert(embedRel, rel)
	}
	r.relToEmbedSrcs[fileRel] = append(r.relToEmbedSrcs[fileRel], embedRel)
}

// resolveDir reads Go files in the directory, resolves //go:embed
// patterns, and stores repo-root-relative paths for files in subdirectories
// into resolvedEmbeds. This is called during Configure (pre-order), so
// when child directories' GenerateRules runs (post-order), they can check
// whether they should generate exports_files rules.
func (r *cachedEmbedResolver) resolveDir(c *config.Config, rel string) {
	di, err := walk.GetDirInfo(rel)
	if err != nil {
		log.Printf("resolveDir: %v", err)
		return
	}

	dir := filepath.Join(c.RepoRoot, rel)

	er := newEmbedResolver(dir, di.Subdirs, di.RegularFiles, di.GenFiles)

	// Parse Go files for embed directives.
	for _, name := range di.RegularFiles {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		// TODO(#2338): goFileInfo incurs extra disk I/O. Think about ways to avoid it.
		info := goFileInfo(filepath.Join(dir, name), rel, path.Join(rel, name))
		fileRel := path.Join(rel, name)
		for _, embed := range info.embeds {
			resolved, err := er.resolve(embed)
			if err != nil {
				log.Print(err)
				continue
			}
			for _, src := range resolved {
				embedRel := path.Join(rel, src)
				r.addEmbedSrc(fileRel, embedRel)
			}
		}
	}
}

// claimExportFiles finds resolved embed sources under the given package prefix
// (rel) that were originated by a different (ancestor) package. It removes
// claimed entries from resolvedEmbeds, records their Bazel labels in
// embedSrcLabels, and returns the package-relative file paths to export.
func (r *cachedEmbedResolver) claimExportFiles(rel string) []string {
	prefix := rel + "/"
	if rel == "" {
		prefix = ""
	}
	var exportFiles []string
	var toDelete []string
	r.resolvedEmbeds.WalkPrefix(prefix, func(embedRel string, v interface{}) bool {
		shallowestEmbedPackage := v.(string)
		// Skip files that belong to the embedder's own package — it doesn't
		// need exports_files for its own embed targets.
		if shallowestEmbedPackage == rel {
			return false
		}
		fileRelToPackage := strings.TrimPrefix(embedRel, prefix)

		toDelete = append(toDelete, embedRel)
		exportFiles = append(exportFiles, fileRelToPackage)
		r.embedSrcLabels[embedRel] = label.Label{Pkg: rel, Name: fileRelToPackage}
		return false
	})
	for _, key := range toDelete {
		r.resolvedEmbeds.Delete(key)
	}
	return exportFiles
}

// Copied from cmd/go/internal/load.validEmbedPattern.
func validEmbedPattern(pattern string) bool {
	return pattern != "." && fsValidPath(pattern)
}

// fsValidPath reports whether the given path name
// is valid for use in a call to Open.
//
// Path names passed to open are UTF-8-encoded,
// unrooted, slash-separated sequences of path elements, like “x/y/z”.
// Path names must not contain an element that is “.” or “..” or the empty string,
// except for the special case that the root directory is named “.”.
// Paths must not start or end with a slash: “/x” and “x/” are invalid.
//
// Note that paths are slash-separated on all systems, even Windows.
// Paths containing other characters such as backslash and colon
// are accepted as valid, but those characters must never be
// interpreted by an FS implementation as path element separators.
//
// Copied from io/fs.ValidPath to avoid making go1.16 a build-time dependency
// for Gazelle.
func fsValidPath(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}

	if name == "." {
		// special case
		return true
	}

	// Iterate over elements in name, checking each.
	for {
		i := 0
		for i < len(name) && name[i] != '/' {
			i++
		}
		elem := name[:i]
		if elem == "" || elem == "." || elem == ".." {
			return false
		}
		if i == len(name) {
			return true // reached clean ending
		}
		name = name[i+1:]
	}
}

// isBadEmbedName reports whether name is the base name of a file that
// can't or won't be included in modules and therefore shouldn't be treated
// as existing for embedding.
//
// Copied from cmd/go/internal/load.isBadEmbedName.
func isBadEmbedName(name string) bool {
	if err := module.CheckFilePath(name); err != nil {
		return true
	}
	switch name {
	// Empty string should be impossible but make it bad.
	case "":
		return true
	// Version control directories won't be present in module.
	case ".bzr", ".hg", ".git", ".svn":
		return true
	}
	return false
}
