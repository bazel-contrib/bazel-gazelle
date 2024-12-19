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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProtoRegexpGroupNames(t *testing.T) {
	names := protoRe.SubexpNames()
	nameMap := map[string]int{
		"import":  importSubexpIndex,
		"package": packageSubexpIndex,
		"optkey":  optkeySubexpIndex,
		"optval":  optvalSubexpIndex,
		"service": serviceSubexpIndex,
		"message": messageSubexpIndex,
		"enum": enumSubexpIndex,
	}
	for name, index := range nameMap {
		if names[index] != name {
			t.Errorf("proto regexp subexp %d is %s ; want %s", index, names[index], name)
		}
	}
	if len(names)-1 != len(nameMap) {
		t.Errorf("proto regexp has %d groups ; want %d", len(names), len(nameMap))
	}
}

func TestProtoFileInfo(t *testing.T) {
	for _, tc := range []struct {
		desc, name, proto string
		want              FileInfo
	}{
		{
			desc:  "empty",
			name:  "empty^file.proto",
			proto: "",
			want:  FileInfo{},
		}, {
			desc:  "simple package",
			name:  "package.proto",
			proto: "package foo;",
			want: FileInfo{
				PackageName: "foo",
			},
		}, {
			desc:  "full package",
			name:  "full.proto",
			proto: "package foo.bar.baz;",
			want: FileInfo{
				PackageName: "foo.bar.baz",
			},
		}, {
			desc: "import simple",
			name: "imp.proto",
			proto: `import 'single.proto';
import "double.proto";`,
			want: FileInfo{
				Imports: []string{"double.proto", "single.proto"},
			},
		}, {
			desc: "import quote",
			name: "quote.proto",
			proto: `import '""\".proto"';
import "'.proto";`,
			want: FileInfo{
				Imports: []string{"\"\"\".proto\"", "'.proto"},
			},
		}, {
			desc:  "import escape",
			name:  "escape.proto",
			proto: `import '\n\012\x0a.proto';`,
			want: FileInfo{
				Imports: []string{"\n\n\n.proto"},
			},
		}, {
			desc: "import two",
			name: "two.proto",
			proto: `import "first.proto";
import "second.proto";`,
			want: FileInfo{
				Imports: []string{"first.proto", "second.proto"},
			},
		}, {
			desc:  "go_package",
			name:  "gopkg.proto",
			proto: `option go_package = "github.com/example/project;projectpb";`,
			want: FileInfo{
				Options: []Option{{Key: "go_package", Value: "github.com/example/project;projectpb"}},
			},
		}, {
			desc:  "service def",
			name:  "service.proto",
			proto: `service ChatService {}`,
			want: FileInfo{
				HasServices: true,
				Services: []string{"ChatService"},
			},
		},
		{
			desc:  "service multiple spaces",
			name:  "service.proto",
			proto: `service      ChatService   {}`,
			want: FileInfo{
				HasServices: true,
				Services: []string{"ChatService"},
			},
		},
		{
			desc:  "service no space for bracket after service name",
			name:  "service.proto",
			proto: `service      ChatService{}`,
			want: FileInfo{
				HasServices: true,
				Services: []string{"ChatService"},
			},
		},
		{
			desc:  "service no space before service name not matched",
			name:  "service.proto",
			proto: `serviceChatService {}`,
			want: FileInfo{
				HasServices: false,
			},
		},
		{
			desc:  "service as name",
			name:  "service.proto",
			proto: `message serviceAccount { string service = 1; }`,
			want: FileInfo{
				HasServices: false,
				Messages: []string{"serviceAccount"},
			},
		},{
			desc: "multiple service names",
			name:  "service.proto",
			proto: `service ServiceA { string service = 1; }

			service    ServiceB    { string service = 1; }

			service ServiceC{ string service = 1; }

			serviceServiceD { string service = 1; }

			service message { string service = 1; }

			service enum { string service = 1; }
			`,
			want: FileInfo{
				HasServices: true,
				Services: []string{"ServiceA", "ServiceB", "ServiceC", "message", "enum"},
			},
		},{
			desc: "multiple message names",
			name:  "messages.proto",
			proto: `message MessageA { string message = 1; }

			message    MessageB    { string message = 1; }

			message MessageC{ string message = 1; }

			messageMessageD { string message = 1; }

			message service { string service = 1; }

			message enum { string service = 1; }
			`,
			want: FileInfo{
				Messages: []string{"MessageA", "MessageB", "MessageC", "service", "enum"},
			},
		},{
			desc: "multiple enum names",
			name:  "enums.proto",
			proto: `enum EnumA {
			    ENUM_VALUE_A = 1;
			    ENUM_VALUE_B = 2;
			}

			enum    EnumB    {
			    ENUM_VALUE_C = 1;
			    ENUM_VALUE_D = 2;
			}

			enum EnumC{
			    ENUM_VALUE_E = 1;
			    ENUM_VALUE_F = 2;
			}

			enumEnumD {
			    ENUM_VALUE_G = 1;
			    ENUM_VALUE_H = 2;
			}

			enum service {
			    ENUM_VALUE_I = 1;
			    ENUM_VALUE_J = 2;
			}

			enum message {
			    ENUM_VALUE_K = 1;
			    ENUM_VALUE_L = 2;
			}
			`,
			want: FileInfo{
				Enums: []string{"EnumA", "EnumB", "EnumC", "service", "message"},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			dir, err := os.MkdirTemp(os.Getenv("TEST_TEMPDIR"), "TestProtoFileinfo")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			if err := os.WriteFile(filepath.Join(dir, tc.name), []byte(tc.proto), 0o600); err != nil {
				t.Fatal(err)
			}

			got := ProtoFileInfo(dir, tc.name)

			// Clear fields we don't care about for testing.
			got = FileInfo{
				PackageName: got.PackageName,
				Imports:     got.Imports,
				Options:     got.Options,
				HasServices: got.HasServices,
				Services:    got.Services,
				Messages:    got.Messages,
				Enums:       got.Enums,
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v; want %#v", got, tc.want)
			}
		})
	}
}
