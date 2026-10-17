// Copyright 2026 The Bazel Authors. All rights reserved.
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

package golang

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

// Each generated configuration input contains the module entries above and
// below its prefix. Watching the nearest existing prefix also detects newly
// added modules that would change resolution of a previously unknown import.
type repositoryConfigInputs struct {
	output   string
	prefixes map[string]bool
	used     map[string]bool
}

func (r *repositoryConfigInputs) init(repos []*rule.Rule) {
	r.prefixes = make(map[string]bool, len(repos))
	r.used = make(map[string]bool)
	for _, repo := range repos {
		if repo.Kind() != "go_repository" {
			continue
		}
		importpath := repo.AttrString("importpath")
		for _, p := range []string{importpath, pathWithoutSemver(importpath)} {
			for p != "" && p != "." && p != "/" && !r.prefixes[p] {
				r.prefixes[p] = true
				p = path.Dir(p)
			}
		}
	}
}

func (r *repositoryConfigInputs) record(importpath string) {
	if r == nil || r.output == "" {
		return
	}
	prefix := strings.TrimSuffix(importpath, "/")
	for !r.prefixes[prefix] && strings.Contains(prefix, "/") {
		prefix = path.Dir(prefix)
	}
	r.used[prefix] = true
}

func (gl *goLang) OnFinish(context.Context) error {
	r := gl.repoConfigInputs
	if r == nil || r.output == "" {
		return nil
	}
	prefixes := make([]string, 0, len(r.used))
	for prefix := range r.used {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	data, err := json.Marshal(prefixes)
	if err != nil {
		return err
	}
	return os.WriteFile(r.output, data, 0o666)
}
