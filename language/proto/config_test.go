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

package proto

import (
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func TestCheckStripImportPrefix(t *testing.T) {
	testCases := []struct {
		name, prefix, rel, wantErr string
	}{
		{
			name:    "not in directory",
			prefix:  "/example.com/idl",
			rel:     "example.com",
			wantErr: `proto_strip_import_prefix "/example.com/idl" not in directory example.com`,
		},
		{
			name:   "strip prefix at root",
			prefix: "/include",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			e := checkStripImportPrefix(tc.prefix, tc.rel)
			if tc.wantErr == "" {
				if e != nil {
					t.Errorf("got:\n%v\n\nwant: nil\n", e)
				}
			} else {
				if e == nil || e.Error() != tc.wantErr {
					t.Errorf("got:\n%v\n\nwant:\n%s\n", e, tc.wantErr)
				}
			}
		})
	}
}

func TestInferProtoMode(t *testing.T) {
	type buildFile struct {
		rel, content, goPrefix, moduleApparentName string
		usingWorkspace                             bool
	}
	type testCase struct {
		desc             string
		build            buildFile
		expected         Mode
		expectedExplicit bool
	}
	for _, tc := range []testCase{
		{
			desc:     "empty build file",
			build:    buildFile{},
			expected: DefaultMode,
		}, {
			desc: "explcit mode",
			build: buildFile{
				content: "# gazelle:proto disable",
			},
			expected:         DisableMode,
			expectedExplicit: true,
		}, {
			desc: "legacy for well known go types prefix",
			build: buildFile{
				goPrefix: wellKnownTypesGoPrefix,
			},
			expected: LegacyMode,
		}, {
			desc: "disabled in vendor",
			build: buildFile{
				rel: "vendor",
			},
			expected: DisableMode,
		},
		{
			desc: "disabled in vendor 2",
			build: buildFile{
				rel: "foo/vendor",
			},
			expected: DisableMode,
		},
		{
			desc: "disabled for custom go_proto_library",
			build: buildFile{
				content: `load("@foreign_rules_go//:custom.bzl", "go_proto_library")`,
			},
			expected: DisableMode,
		},
		{
			desc: "default for rules_go:go_proto_library",
			build: buildFile{
				content: `load("@rules_go//proto:def.bzl", "go_proto_library")`,
			},
			expected: DefaultMode,
		},
		{
			desc: "default for workspace rules_go:go_proto_library",
			build: buildFile{
				content:        `load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")`,
				usingWorkspace: true,
			},
			expected: DefaultMode,
		},
		{
			desc: "default for apparent rules_go:go_proto_library",
			build: buildFile{
				content:            `load("@custom_rules_go//proto:def.bzl", "go_proto_library")`,
				moduleApparentName: "custom_rules_go",
			},
			expected: DefaultMode,
		},
		{
			desc: "legacy for rules_go go_proto_library",
			build: buildFile{
				content: `load("@rules_go//proto:go_proto_library.bzl", "go_proto_library")`,
			},
			expected: LegacyMode,
		},
		{
			desc: "legacy for apparent rules_go go_proto_library",
			build: buildFile{
				content:            `load("@custom_rules_go//proto:go_proto_library.bzl", "go_proto_library")`,
				moduleApparentName: "custom_rules_go",
			},
			expected: LegacyMode,
		},
		{
			desc: "disabled for unknown rules_go go_proto_library",
			build: buildFile{
				content: `load("@foreign_rules_go//proto:go_proto_library.bzl", "go_proto_library")`,
			},
			expected: DisableMode,
		},
	} {
		file, err := rule.LoadData(filepath.Join(tc.build.rel, "BUILD.bazel"), tc.build.rel, []byte(tc.build.content))
		if err != nil {
			t.Fatal(err)
		}
		// Set up the minimal config for the test, we populate only with the data required for the protoConfig mode inferring logic.
		config := config.New()
		config.Exts[protoName] = &ProtoConfig{GoPrefix: tc.build.goPrefix}
		config.ModuleToApparentName = func(module string) string {
			if tc.build.usingWorkspace {
				return ""
			}
			switch module {
			case "rules_go":
				if tc.build.moduleApparentName != "" {
					return tc.build.moduleApparentName
				}
				return module
			default:
				return ""
			}
		}
		NewLanguage().Configure(config, tc.build.rel, file)
		pc := GetProtoConfig(config)
		if pc.Mode != tc.expected {
			t.Errorf("for %q, got mode %v, want %v", tc.desc, pc.Mode, tc.expected)
		}
		if pc.ModeExplicit != tc.expectedExplicit {
			t.Errorf("for %q, got mode explicit %v, want %v", tc.desc, pc.ModeExplicit, tc.expectedExplicit)
		}
	}
}
