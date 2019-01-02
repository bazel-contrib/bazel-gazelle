/* Copyright 2017 The Bazel Authors. All rights reserved.

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

package repo

import (
	"io/ioutil"

	"github.com/bazelbuild/bazel-gazelle/label"
	toml "github.com/pelletier/go-toml"
)

type depLockFile struct {
	Projects []depProject `toml:"projects"`
}

type depProject struct {
	Name     string `toml:"name"`
	Revision string `toml:"revision"`
	Source   string `toml:"source"`
}

func importRepoRulesDep(filename string) ([]Repo, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var file depLockFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	var repos []Repo
	for _, p := range file.Projects {
		var vcs string
		if p.Source != "" {
			// TODO(#411): Handle source directives correctly. It may be an import
			// path, or a URL. In the case of an import path, we should resolve it
			// to the correct remote and vcs. In the case of a URL, we should
			// correctly determine what VCS to use (the URL will usually start
			// with "https://", which is used by multiple VCSs).
			vcs = "git"
		}
		repos = append(repos, Repo{
			Name:     label.ImportPathToBazelRepoName(p.Name),
			GoPrefix: p.Name,
			Commit:   p.Revision,
			Remote:   p.Source,
			VCS:      vcs,
		})
	}
	return repos, nil
}
