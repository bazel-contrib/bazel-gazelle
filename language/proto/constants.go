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

const (
	// PackageInfoKey is the name of a private attribute set on generated
	// proto_library rules. This attribute contains a Package record which
	// describes the library and its sources.
	PackageKey = "_package"

	// wellKnownTypesGoPrefix is the import path for the Go repository containing
	// pre-generated code for the Well Known Types.
	wellKnownTypesGoPrefix = "github.com/golang/protobuf"
)

// bazelModuleRepos maps the names of Bazel modules to their "well-known"
// WORKSPACE repository name
var bazelModuleRepos = map[string]string{
	"protobuf":    "com_google_protobuf",
	"rules_proto": "rules_proto", // Deprecated, kept for backwards compatibility
}
