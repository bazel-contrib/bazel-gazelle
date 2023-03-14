/* Copyright 2019 The Bazel Authors. All rights reserved.

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

// Command generate_repo_config takes in a build config file such as
// WORKSPACE and generates a stripped version of the file. The generated
// config file should contain only the information relevant to gazelle for
// dependency resolution, so go_repository rules with importpath
// and name defined, plus any directives.
//
// This command is used by the go_repository_config rule to generate a repo
// config file used by all go_repository rules. A list of macro files is
// printed to stdout to be read by the go_repository_config rule.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const (
	goRepoRuleKind = "go_repository"
)

var (
	configSource = flag.String("config_source", "", "a file that is read to learn about external repositories")
	configDest   = flag.String("config_dest", "", "destination file for the generated repo config")
)

type byName []*rule.Rule

func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func main() {
	log.SetFlags(0)
	log.SetPrefix("generate_repo_config: ")

	flag.Parse()
	if *configDest == "" {
		log.Fatal("-config_dest must be set")
	}
	if *configSource == "" {
		log.Fatal("-config_source must be set")
	}
	if flag.NArg() != 0 {
		log.Fatal("generate_repo_config does not accept positional arguments")
	}
	files, err := generateRepoConfig(*configDest, *configSource)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		fmt.Fprintln(os.Stdout, f)
	}
}

func generateRepoConfig(configDest, configSource string) ([]string, error) {
	var buf bytes.Buffer
	buf.WriteString("# Code generated by generate_repo_config.go; DO NOT EDIT.\n")

	sourceFile, err := rule.LoadWorkspaceFile(configSource, "")
	if err != nil {
		return nil, err
	}
	repos, repoFileMap, err := repo.ListRepositories(sourceFile)
	if err != nil {
		return nil, err
	}
	sort.Stable(byName(repos))

	seenFile := make(map[*rule.File]bool)
	var sortedFiles []*rule.File
	for _, f := range repoFileMap {
		if !seenFile[f] {
			seenFile[f] = true
			sortedFiles = append(sortedFiles, f)
		}
	}
	sort.SliceStable(sortedFiles, func(i, j int) bool {
		if cmp := strings.Compare(sortedFiles[i].Path, sortedFiles[j].Path); cmp != 0 {
			return cmp < 0
		}
		return sortedFiles[i].DefName < sortedFiles[j].DefName
	})

	destFile := rule.EmptyFile(configDest, "")
	for _, rsrc := range repos {
		if rsrc.Kind() != goRepoRuleKind {
			continue
		}

		rdst := rule.NewRule(goRepoRuleKind, rsrc.Name())
		rdst.SetAttr("importpath", rsrc.AttrString("importpath"))
		if namingConvention := rsrc.AttrString("build_naming_convention"); namingConvention != "" {
			rdst.SetAttr("build_naming_convention", namingConvention)
		}
		rdst.Insert(destFile)
	}

	buf.WriteString("\n")
	buf.Write(destFile.Format())
	if err := ioutil.WriteFile(configDest, buf.Bytes(), 0o666); err != nil {
		return nil, err
	}

	files := make([]string, 0, len(sortedFiles))
	for _, m := range sortedFiles {
		// We have to trim the configSource file path from the repo files returned.
		// This is safe/required because repo.ListRepositories(sourceFile) is called
		// with the sourcefile as the workspace, so the source file location is always
		// prepended to the macro file paths.
		// TODO: https://github.com/bazelbuild/bazel-gazelle/issues/1068
		f, err := filepath.Rel(filepath.Dir(configSource), m.Path)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, nil
}
