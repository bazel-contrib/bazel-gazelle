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

package update

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// BazelModuleVersion is the version of the Gazelle Bazel module. It may be used
// to change behavior across versions built from the same code.
var BazelModuleVersion string

// IsBazelModule is set to a value that parses to "true" if Gazelle was built by
// Bazel in module mode.
var IsBazelModule string

// MajorVersion is set by v2/cmd/gazelle to determine global behavior that
// differs between v1 and v2. Assume v1 behavior if not set.
var MajorVersion int

// errVersion is a special value indicating the -version flag was set, and the
// version was printed. Run recovers from this by doing nothing and
// returning nil.
var errVersion = errors.New("version printed")

func printVersion(knownLanguages []string) {
	if BazelModuleVersion == "" {
		fmt.Printf("gazelle version unknown\n")
	} else {
		fmt.Printf("gazelle %s\n", BazelModuleVersion)
	}
	if moduleMode, _ := strconv.ParseBool(IsBazelModule); moduleMode {
		fmt.Printf("built in module mode\n")
	} else {
		fmt.Printf("built in workspace mode\n")
	}
	fmt.Printf("language extensions: %s\n", strings.Join(knownLanguages, ", "))
}
