/* Copyright 2022 The Bazel Authors. All rights reserved.

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
	"path/filepath"

	languagev1 "github.com/bazelbuild/bazel-gazelle/language"
)

func importReposFromWork(args languagev1.ImportReposArgs) languagev1.ImportReposResult {
	// run go list in the dir where go.work is located
	data, err := goListModules(filepath.Dir(args.Path))
	if err != nil {
		return languagev1.ImportReposResult{Error: processGoListError(nil, data)}
	}

	pathToModule, err := extractModules(data)
	if err != nil {
		return languagev1.ImportReposResult{Error: err}
	}

	pathToModule, err = fillMissingSums(pathToModule)
	if err != nil {
		return languagev1.ImportReposResult{Error: fmt.Errorf("finding module sums: %v", err)}
	}

	return languagev1.ImportReposResult{Gen: toRepositoryRules(pathToModule)}
}
