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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bazel-contrib/bazel-gazelle/v2/cmd/gazelle/update"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/testtools"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
)

func TestRepositoryConfigInputs(t *testing.T) {
	for _, tc := range []struct {
		name, source, importpath, prefix string
		want                             []string
	}{
		{"external", "lib.go", "example.com/dep/pkg", "", []string{"example.com/consumer", "example.com/dep"}},
		{"unknown", "lib.go", "unknown.dev/pkg", "", []string{"example.com/consumer", "unknown.dev"}},
		{"visibility", "internal/lib/lib.go", "fmt", "", []string{"example.com/consumer"}},
		{"prefix_override", "internal/lib/lib.go", "fmt", "", []string{"example.com/consumer"}},
		{
			"internal_module", "lib.go", "fmt", "example.com/consumer/internal/submodule",
			[]string{"example.com/consumer", "example.com/consumer/internal/submodule"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := tc.prefix
			if prefix == "" {
				prefix = "example.com/consumer"
			}
			dir, cleanup := testtools.CreateFiles(t, []testtools.FileSpec{
				{Path: "MODULE.bazel"},
				{Path: "BUILD.bazel"},
				{Path: tc.source, Content: "package lib\nimport _ \"" + tc.importpath + "\"\n"},
			})
			defer cleanup()
			if tc.name == "prefix_override" {
				if err := os.WriteFile(filepath.Join(dir, "BUILD.bazel"), []byte("# gazelle:prefix example.com/consumer/child\n"), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			configPath := filepath.Join(dir, "repos.WORKSPACE")
			inputsPath := filepath.Join(dir, ".config_inputs.json")
			config := fmt.Sprintf(`go_repository(name = "consumer", importpath = %q)
go_repository(name = "dep", importpath = "example.com/dep")
go_repository(name = "child", importpath = "example.com/consumer/child")
`, prefix)
			if err := os.WriteFile(configPath, []byte(config), 0o666); err != nil {
				t.Fatal(err)
			}
			if err := update.Run(t.Context(), []language.Language{proto.NewV2(), NewV2()}, dir, []string{
				"-go_repository_mode", "-go_prefix=" + prefix, "-external=static",
				"-repo_config=" + configPath, "-go_repository_config_inputs=" + inputsPath,
			}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(inputsPath)
			if err != nil {
				t.Fatal(err)
			}
			var inputs []string
			if err := json.Unmarshal(data, &inputs); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(inputs, tc.want) {
				t.Fatalf("configuration inputs = %v, want %v", inputs, tc.want)
			}
		})
	}
}
