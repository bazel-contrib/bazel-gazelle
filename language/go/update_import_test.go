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

package golang

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/testtools"
)

func TestImports(t *testing.T) {
	for _, tc := range []struct {
		desc, want        string
		wantErr           string
		stubGoModDownload func(string, []string) ([]byte, error)
		stubGoListModules func(string) ([]byte, error)
		files             []testtools.FileSpec
	}{
		{
			desc: "modules",
			files: []testtools.FileSpec{
				{
					Path: "go.mod",
					Content: `
module github.com/bazelbuild/bazel-gazelle

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/bazelbuild/buildtools v0.0.0-20190202002759-027686e28d67
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fork/go-toml v0.0.0-20190116191733-b6c0e53d7304
	github.com/fsnotify/fsnotify v1.4.7
	github.com/kr/pretty v0.1.0 // indirect
	github.com/pelletier/go-toml v1.0.1
	github.com/pmezard/go-difflib v1.0.0
	golang.org/x/sys v0.0.0-20190122071731-054c452bb702 // indirect
	golang.org/x/tools v0.0.0-20190122202912-9c309ee22fab
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
)

replace (
	github.com/pelletier/go-toml => github.com/fork/go-toml v0.0.0-20190425002759-70bc0436ed16
)
`,
				}, {
					// Note: the sum for x/tools has been deleted to force a call
					// to goModDownload.
					Path: "go.sum",
					Content: `
github.com/BurntSushi/toml v0.3.1 h1:WXkYYl6Yr3qBf1K79EBnL4mak0OimBfB0XUf9Vl28OQ=
github.com/BurntSushi/toml v0.3.1/go.mod h1:xHWCNGjB5oqiDr8zfno3MHue2Ht5sIBksp03qcyfWMU=
github.com/bazelbuild/buildtools v0.0.0-20190202002759-027686e28d67 h1:zS8p6ZRbNVa7QfK3tpoIRDqGzCA2J0uJffaMTWoneac=
github.com/bazelbuild/buildtools v0.0.0-20190202002759-027686e28d67/go.mod h1:5JP0TXzWDHXv8qvxRC4InIazwdyDseBDbzESUMKk1yU=
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/fsnotify/fsnotify v1.4.7 h1:IXs+QLmnXW2CcXuY+8Mzv/fWEsPGWxqefPtCP5CnV9I=
github.com/fsnotify/fsnotify v1.4.7/go.mod h1:jwhsz4b93w/PPRr/qN1Yymfu8t87LnFCMoQvtojpjFo=
github.com/kr/pretty v0.1.0 h1:L/CwN0zerZDmRFUapSPitk6f+Q3+0za1rQkzVuMiMFI=
github.com/kr/pretty v0.1.0/go.mod h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo=
github.com/kr/pty v1.1.1/go.mod h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ=
github.com/kr/text v0.1.0 h1:45sCR5RtlFHMR4UwH9sdQ5TC8v0qDQCHnXt+kaKSTVE=
github.com/kr/text v0.1.0/go.mod h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI=
github.com/fork/go-toml v0.0.0-20190116191733-b6c0e53d7304 h1:5+8j8FTpnFV4nEImW/ofkzEt8VoOiLXxdYIDsB73T38=
github.com/fork/go-toml v0.0.0-20190116191733-b6c0e53d7304/go.mod h1:ZiWeW+zYFKm7srdB9IoDzzZXaJaI5eL9QjNiN/DMA2s=
github.com/fork/go-toml v0.0.0-20190425002759-70bc0436ed16 h1:T5zMGML61Wp+FlcbWjRDT7yAxhJNAiPPLOFECq181zc=
github.com/fork/go-toml v0.0.0-20190425002759-70bc0436ed16/go.mod h1:5z9KED0ma1S8pY6P1sdut58dfprrGBbd/94hg7ilaic=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
golang.org/x/sys v0.0.0-20190122071731-054c452bb702 h1:Lk4tbZFnlyPgV+sLgTw5yGfzrlOn9kx4vSombi2FFlY=
golang.org/x/sys v0.0.0-20190122071731-054c452bb702/go.mod h1:STP8DvDyc/dI5b8T5hshtkjS+E42TnysNCUPdjciGhY=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405 h1:yhCVgyC4o1eVCa2tZl7eS0r+SDo693bJlVdllGtEeKM=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 h1:qIbj1fsPNlZgppZ+VLlY7N33q108Sa+fhmuc+sWQYwY=
gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/yaml.v2 v2.2.2 h1:ZCJp+EgiOT7lHqUV2J862kp8Qj64Jo6az82+3Td9dZw=
gopkg.in/yaml.v2 v2.2.2/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
`,
				},
			},
			want: `
go_repository(
    name = "com_github_bazelbuild_buildtools",
    importpath = "github.com/bazelbuild/buildtools",
    sum = "h1:zS8p6ZRbNVa7QfK3tpoIRDqGzCA2J0uJffaMTWoneac=",
    version = "v0.0.0-20190202002759-027686e28d67",
)

go_repository(
    name = "com_github_burntsushi_toml",
    importpath = "github.com/BurntSushi/toml",
    sum = "h1:WXkYYl6Yr3qBf1K79EBnL4mak0OimBfB0XUf9Vl28OQ=",
    version = "v0.3.1",
)

go_repository(
    name = "com_github_davecgh_go_spew",
    importpath = "github.com/davecgh/go-spew",
    sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_fork_go_toml",
    importpath = "github.com/fork/go-toml",
    sum = "h1:5+8j8FTpnFV4nEImW/ofkzEt8VoOiLXxdYIDsB73T38=",
    version = "v0.0.0-20190116191733-b6c0e53d7304",
)

go_repository(
    name = "com_github_pelletier_go_toml",
    importpath = "github.com/pelletier/go-toml",
    replace = "github.com/fork/go-toml",
    sum = "h1:T5zMGML61Wp+FlcbWjRDT7yAxhJNAiPPLOFECq181zc=",
    version = "v0.0.0-20190425002759-70bc0436ed16",
)

go_repository(
    name = "in_gopkg_check_v1",
    importpath = "gopkg.in/check.v1",
    sum = "h1:qIbj1fsPNlZgppZ+VLlY7N33q108Sa+fhmuc+sWQYwY=",
    version = "v1.0.0-20180628173108-788fd7840127",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    importpath = "gopkg.in/yaml.v2",
    sum = "h1:ZCJp+EgiOT7lHqUV2J862kp8Qj64Jo6az82+3Td9dZw=",
    version = "v2.2.2",
)

go_repository(
    name = "org_golang_x_tools",
    importpath = "golang.org/x/tools",
    sum = "h1:FkAkwuYWQw+IArrnmhGlisKHQF4MsZ2Nu/fX4ttW55o=",
    version = "v0.0.0-20190122202912-9c309ee22fab",
)
`,
			wantErr:           "",
			stubGoModDownload: nil,
		},
		{
			desc: "modules-download-error",
			files: []testtools.FileSpec{
				{
					Path: "go.mod",
					Content: `
module github.com/bazelbuild/bazel-gazelle

require (
	definitely.doesnotexist/ever v0.1.0
)
`,
				}, {
					Path: "go.sum",
					Content: `
definitely.doesnotexist/ever v0.1.0/go.mod h1:HI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuJ=
`,
				},
			},
			want:    "",
			wantErr: "finding module sums: error from go mod download: failed to download\nError downloading definitely.doesnotexist/ever: Did not exist",
			stubGoModDownload: func(dir string, args []string) ([]byte, error) {
				return []byte(`{
"Path": "definitely.doesnotexist/ever",
"Version": "0.1.0",
"Error": "Did not exist"
}`), fmt.Errorf("failed to download")
			},
		},
		{
			desc: "modules-download-bad-json",
			files: []testtools.FileSpec{
				{
					Path: "go.mod",
					Content: `
module github.com/bazelbuild/bazel-gazelle

require (
	definitely.doesnotexist/ever v0.1.0
)
`,
				}, {
					Path: "go.sum",
					Content: `
definitely.doesnotexist/ever v0.1.0/go.mod h1:HI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuJ=
`,
				},
			},
			want:    "",
			wantErr: "finding module sums: error from go mod download: failed to download\nError parsing module for more error information: invalid character 'o' in literal null (expecting 'u')",
			stubGoModDownload: func(dir string, args []string) ([]byte, error) {
				return []byte(`{
"Path": "definitely.doesnotexist/ever",
"Version": "0.1.0",
"Error": {
   "Err": not valid json
}
}`), fmt.Errorf("failed to download")
			},
		},
		{
			desc: "list-modules-error",
			files: []testtools.FileSpec{
				{
					Path: "go.mod",
					Content: `
module github.com/bazelbuild/bazel-gazelle

require (
	definitely.doesnotexist/ever v0.1.0
)
`,
				}, {
					Path: "go.sum",
					Content: `
definitely.doesnotexist/ever v0.1.0/go.mod h1:HI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuJ=
`,
				},
			},
			want:    "",
			wantErr: "error from go list: failed to download\nError listing definitely.doesnotexist/ever: Did not exist",
			stubGoListModules: func(dir string) ([]byte, error) {
				return []byte(`{
"Path": "definitely.doesnotexist/ever",
"Version": "0.1.0",
"Error": {"Err": "Did not exist"}
}`), fmt.Errorf("failed to download")
			},
		},
		{
			desc: "list-modules-bad-json",
			files: []testtools.FileSpec{
				{
					Path: "go.mod",
					Content: `
module github.com/bazelbuild/bazel-gazelle

require (
	definitely.doesnotexist/ever v0.1.0
)
`,
				}, {
					Path: "go.sum",
					Content: `
definitely.doesnotexist/ever v0.1.0/go.mod h1:HI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuJ=
`,
				},
			},
			want:    "",
			wantErr: "error from go list: failed to download\nError parsing module for more error information: invalid character 'n' after object key",
			stubGoListModules: func(dir string) ([]byte, error) {
				return []byte(`{
    "Path": "definitely.doesnotexist/ever",
    "Version": "0.1.0",
    "Error" not valid json
}`), fmt.Errorf("failed to download")
			},
		},
		{
			desc: "work",
			files: []testtools.FileSpec{
				{
					Path: "go.work",
					Content: `
go 1.18

use (
	./project1
	./project2
	./project3
)
`,
				},
				{
					Path: "project1/go.mod",
					Content: `
module project1

go 1.18

require github.com/vmware/govmomi v0.21.0 // indirect
`,
				},
				{
					Path: "project1/go.sum",
					Content: `
github.com/davecgh/go-xdr v0.0.0-20161123171359-e6a2ba005892/go.mod h1:CTDl0pzVzE5DEzZhPfvhY/9sPFMQIxaJ9VAMs9AagrE=
github.com/google/uuid v0.0.0-20170306145142-6a5e28554805/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/kr/pretty v0.1.0/go.mod h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo=
github.com/kr/pty v1.1.1/go.mod h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ=
github.com/kr/text v0.1.0/go.mod h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI=
github.com/vmware/govmomi v0.21.0 h1:jc8uMuxpcV2xMAA/cnEDlnsIjvqcMra5Y8onh/U3VuY=
github.com/vmware/govmomi v0.21.0/go.mod h1:zbnFoBQ9GIjs2RVETy8CNEpb+L+Lwkjs3XZUL0B3/m0=
github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728/go.mod h1:x9oS4Wk2s2u4tS29nEaDLdzvuHdB19CvSGJjPgkZJNk=
`,
				},
				{
					Path: "project2/go.mod",
					Content: `
module project2

go 1.18

require github.com/vmware/govmomi v0.24.0 // indirect
`,
				},
				{
					Path: "project2/go.sum",
					Content: `
github.com/davecgh/go-xdr v0.0.0-20161123171359-e6a2ba005892/go.mod h1:CTDl0pzVzE5DEzZhPfvhY/9sPFMQIxaJ9VAMs9AagrE=
github.com/google/uuid v0.0.0-20170306145142-6a5e28554805/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/kr/pretty v0.1.0/go.mod h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo=
github.com/kr/pty v1.1.1/go.mod h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ=
github.com/kr/text v0.1.0/go.mod h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI=
github.com/vmware/govmomi v0.24.0 h1:G7YFF6unMTG3OY25Dh278fsomVTKs46m2ENlEFSbmbs=
github.com/vmware/govmomi v0.24.0/go.mod h1:Y+Wq4lst78L85Ge/F8+ORXIWiKYqaro1vhAulACy9Lc=
github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728/go.mod h1:x9oS4Wk2s2u4tS29nEaDLdzvuHdB19CvSGJjPgkZJNk=
`,
				},
				{
					Path: "project3/go.mod",
					Content: `
module project3

go 1.18

require github.com/vmware/govmomi v0.27.0 // indirect
`,
				},
				{
					Path: "project3/go.sum",
					Content: `
github.com/a8m/tree v0.0.0-20210115125333-10a5fd5b637d/go.mod h1:FSdwKX97koS5efgm8WevNf7XS3PqtyFkKDDXrz778cg=
github.com/davecgh/go-xdr v0.0.0-20161123171359-e6a2ba005892/go.mod h1:CTDl0pzVzE5DEzZhPfvhY/9sPFMQIxaJ9VAMs9AagrE=
github.com/google/uuid v1.2.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/kr/pretty v0.1.0/go.mod h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo=
github.com/kr/pty v1.1.1/go.mod h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ=
github.com/kr/text v0.1.0/go.mod h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI=
github.com/vmware/govmomi v0.27.0 h1:KoQ8IsLAa7V78s5d7dgpZA8d039GBM83cVxgAq9uWuw=
github.com/vmware/govmomi v0.27.0/go.mod h1:daTuJEcQosNMXYJOeku0qdBJP9SOLLWB3Mqz8THtv6o=
github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728/go.mod h1:x9oS4Wk2s2u4tS29nEaDLdzvuHdB19CvSGJjPgkZJNk=
`,
				},
			},
			want: `
go_repository(
    name = "com_github_a8m_tree",
    importpath = "github.com/a8m/tree",
    sum = "h1:4E8RufAN3UQ/weB6AnQ4y5miZCO0Yco8ZdGId41WuQs=",
    version = "v0.0.0-20210115125333-10a5fd5b637d",
)

go_repository(
    name = "com_github_davecgh_go_xdr",
    importpath = "github.com/davecgh/go-xdr",
    sum = "h1:qg9VbHo1TlL0KDM0vYvBG9EY0X0Yku5WYIPoFWt8f6o=",
    version = "v0.0.0-20161123171359-e6a2ba005892",
)

go_repository(
    name = "com_github_google_uuid",
    importpath = "github.com/google/uuid",
    sum = "h1:qJYtXnJRWmpe7m/3XlyhrsLrEURqHRM2kxzoxXqyUDs=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_kr_pretty",
    importpath = "github.com/kr/pretty",
    sum = "h1:L/CwN0zerZDmRFUapSPitk6f+Q3+0za1rQkzVuMiMFI=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_kr_pty",
    importpath = "github.com/kr/pty",
    sum = "h1:VkoXIwSboBpnk99O/KFauAEILuNHv5DVFKZMBN/gUgw=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_kr_text",
    importpath = "github.com/kr/text",
    sum = "h1:45sCR5RtlFHMR4UwH9sdQ5TC8v0qDQCHnXt+kaKSTVE=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_vmware_govmomi",
    importpath = "github.com/vmware/govmomi",
    sum = "h1:KoQ8IsLAa7V78s5d7dgpZA8d039GBM83cVxgAq9uWuw=",
    version = "v0.27.0",
)

go_repository(
    name = "com_github_vmware_vmw_guestinfo",
    importpath = "github.com/vmware/vmw-guestinfo",
    sum = "h1:sH9mEk+flyDxiUa5BuPiuhDETMbzrt9A20I2wktMvRQ=",
    version = "v0.0.0-20170707015358-25eff159a728",
)
`,
			wantErr: "",
			stubGoModDownload: func(s string, i []string) ([]byte, error) {
				return []byte(`
{
	"Path": "github.com/a8m/tree",
	"Version": "v0.0.0-20210115125333-10a5fd5b637d",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/a8m/tree/@v/v0.0.0-20210115125333-10a5fd5b637d.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/a8m/tree/@v/v0.0.0-20210115125333-10a5fd5b637d.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/a8m/tree/@v/v0.0.0-20210115125333-10a5fd5b637d.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/a8m/tree@v0.0.0-20210115125333-10a5fd5b637d",
	"Sum": "h1:4E8RufAN3UQ/weB6AnQ4y5miZCO0Yco8ZdGId41WuQs=",
	"GoModSum": "h1:FSdwKX97koS5efgm8WevNf7XS3PqtyFkKDDXrz778cg="
}
{
	"Path": "github.com/davecgh/go-xdr",
	"Version": "v0.0.0-20161123171359-e6a2ba005892",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/davecgh/go-xdr/@v/v0.0.0-20161123171359-e6a2ba005892.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/davecgh/go-xdr/@v/v0.0.0-20161123171359-e6a2ba005892.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/davecgh/go-xdr/@v/v0.0.0-20161123171359-e6a2ba005892.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/davecgh/go-xdr@v0.0.0-20161123171359-e6a2ba005892",
	"Sum": "h1:qg9VbHo1TlL0KDM0vYvBG9EY0X0Yku5WYIPoFWt8f6o=",
	"GoModSum": "h1:CTDl0pzVzE5DEzZhPfvhY/9sPFMQIxaJ9VAMs9AagrE="
}
{
	"Path": "github.com/google/uuid",
	"Version": "v1.2.0",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/google/uuid/@v/v1.2.0.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/google/uuid/@v/v1.2.0.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/google/uuid/@v/v1.2.0.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/google/uuid@v1.2.0",
	"Sum": "h1:qJYtXnJRWmpe7m/3XlyhrsLrEURqHRM2kxzoxXqyUDs=",
	"GoModSum": "h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo="
}
{
	"Path": "github.com/kr/pretty",
	"Version": "v0.1.0",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pretty/@v/v0.1.0.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pretty/@v/v0.1.0.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pretty/@v/v0.1.0.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/kr/pretty@v0.1.0",
	"Sum": "h1:L/CwN0zerZDmRFUapSPitk6f+Q3+0za1rQkzVuMiMFI=",
	"GoModSum": "h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo="
}
{
	"Path": "github.com/kr/pty",
	"Version": "v1.1.1",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pty/@v/v1.1.1.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pty/@v/v1.1.1.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pty/@v/v1.1.1.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/kr/pty@v1.1.1",
	"Sum": "h1:VkoXIwSboBpnk99O/KFauAEILuNHv5DVFKZMBN/gUgw=",
	"GoModSum": "h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ="
}
{
	"Path": "github.com/kr/text",
	"Version": "v0.1.0",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/text/@v/v0.1.0.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/text/@v/v0.1.0.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/text/@v/v0.1.0.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/kr/text@v0.1.0",
	"Sum": "h1:45sCR5RtlFHMR4UwH9sdQ5TC8v0qDQCHnXt+kaKSTVE=",
	"GoModSum": "h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI="
}
{
	"Path": "github.com/vmware/govmomi",
	"Version": "v0.27.0",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/govmomi/@v/v0.27.0.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/govmomi/@v/v0.27.0.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/govmomi/@v/v0.27.0.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/vmware/govmomi@v0.27.0",
	"Sum": "h1:KoQ8IsLAa7V78s5d7dgpZA8d039GBM83cVxgAq9uWuw=",
	"GoModSum": "h1:daTuJEcQosNMXYJOeku0qdBJP9SOLLWB3Mqz8THtv6o="
}
{
	"Path": "github.com/vmware/vmw-guestinfo",
	"Version": "v0.0.0-20170707015358-25eff159a728",
	"Info": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/vmw-guestinfo/@v/v0.0.0-20170707015358-25eff159a728.info",
	"GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/vmw-guestinfo/@v/v0.0.0-20170707015358-25eff159a728.mod",
	"Zip": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/vmw-guestinfo/@v/v0.0.0-20170707015358-25eff159a728.zip",
	"Dir": "/Users/hhalil/go/pkg/mod/github.com/vmware/vmw-guestinfo@v0.0.0-20170707015358-25eff159a728",
	"Sum": "h1:sH9mEk+flyDxiUa5BuPiuhDETMbzrt9A20I2wktMvRQ=",
	"GoModSum": "h1:x9oS4Wk2s2u4tS29nEaDLdzvuHdB19CvSGJjPgkZJNk="
}
`), nil
			},
			stubGoListModules: func(dir string) ([]byte, error) {
				return []byte(`
{
        "Path": "project1",
        "Main": true,
        "Dir": "/Users/hhalil/Documents/VMware/mirrors_github_bazel-gazelle/hakan/project1",
        "GoMod": "/Users/hhalil/Documents/VMware/mirrors_github_bazel-gazelle/hakan/project1/go.mod",
        "GoVersion": "1.18"
}
{
        "Path": "project2",
        "Main": true,
        "Dir": "/Users/hhalil/Documents/VMware/mirrors_github_bazel-gazelle/hakan/project2",
        "GoMod": "/Users/hhalil/Documents/VMware/mirrors_github_bazel-gazelle/hakan/project2/go.mod",
        "GoVersion": "1.18"
}
{
        "Path": "project3",
        "Main": true,
        "Dir": "/Users/hhalil/Documents/VMware/mirrors_github_bazel-gazelle/hakan/project3",
        "GoMod": "/Users/hhalil/Documents/VMware/mirrors_github_bazel-gazelle/hakan/project3/go.mod",
        "GoVersion": "1.18"
}
{
        "Path": "github.com/a8m/tree",
        "Version": "v0.0.0-20210115125333-10a5fd5b637d",
        "Time": "2021-01-15T12:53:33Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/a8m/tree/@v/v0.0.0-20210115125333-10a5fd5b637d.mod"
}
{
        "Path": "github.com/davecgh/go-xdr",
        "Version": "v0.0.0-20161123171359-e6a2ba005892",
        "Time": "2016-11-23T17:13:59Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/davecgh/go-xdr/@v/v0.0.0-20161123171359-e6a2ba005892.mod"
}
{
        "Path": "github.com/google/uuid",
        "Version": "v1.2.0",
        "Time": "2021-01-22T18:20:15Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/google/uuid/@v/v1.2.0.mod"
}
{
        "Path": "github.com/kr/pretty",
        "Version": "v0.1.0",
        "Time": "2018-05-06T08:33:45Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pretty/@v/v0.1.0.mod"
}
{
        "Path": "github.com/kr/pty",
        "Version": "v1.1.1",
        "Time": "2018-01-13T18:08:13Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/pty/@v/v1.1.1.mod"
}
{
        "Path": "github.com/kr/text",
        "Version": "v0.1.0",
        "Time": "2018-05-06T08:24:08Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/kr/text/@v/v0.1.0.mod"
}
{
        "Path": "github.com/vmware/govmomi",
        "Version": "v0.27.0",
        "Time": "2021-10-14T20:30:09Z",
        "Indirect": true,
        "Dir": "/Users/hhalil/go/pkg/mod/github.com/vmware/govmomi@v0.27.0",
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/govmomi/@v/v0.27.0.mod",
        "GoVersion": "1.14"
}
{
        "Path": "github.com/vmware/vmw-guestinfo",
        "Version": "v0.0.0-20170707015358-25eff159a728",
        "Time": "2017-07-07T01:53:58Z",
        "Indirect": true,
        "GoMod": "/Users/hhalil/go/pkg/mod/cache/download/github.com/vmware/vmw-guestinfo/@v/v0.0.0-20170707015358-25eff159a728.mod"
}
`), nil
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.stubGoModDownload != nil {
				previousGoModDownload := goModDownload
				goModDownload = tc.stubGoModDownload
				defer func() { goModDownload = previousGoModDownload }()
			}
			if tc.stubGoListModules != nil {
				previousGoListModules := goListModules
				goListModules = tc.stubGoListModules
				defer func() { goListModules = previousGoListModules }()
			}
			dir, cleanup := testtools.CreateFiles(t, tc.files)
			defer cleanup()

			filename := filepath.Join(dir, tc.files[0].Path)
			c := &config.Config{Exts: map[string]interface{}{}}
			rc, rcCleanup := repo.NewRemoteCache(nil)
			defer func() {
				if err := rcCleanup(); err != nil {
					t.Fatal(err)
				}
			}()
			gl := NewLanguage()
			gl.Configure(c, "", nil)
			importer := gl.(language.RepoImporter)
			result := importer.ImportRepos(language.ImportReposArgs{
				Config: c,
				Path:   filename,
				Cache:  rc,
			})
			if tc.wantErr != "" {
				if result.Error == nil {
					t.Fatalf("Want error %v but got %v", tc.wantErr, result)
				}
				if result.Error.Error() != tc.wantErr {
					t.Fatalf("Want error %v but got %v", tc.wantErr, result.Error)
				}
				return
			} else {
				if result.Error != nil {
					t.Fatal(result.Error)
				}
			}
			f := rule.EmptyFile("test", "")
			for _, r := range result.Gen {
				r.Insert(f)
			}
			got := strings.TrimSpace(string(f.Format()))
			want := strings.TrimSpace(tc.want)
			if got != want {
				t.Errorf("got:\n%s\n\nwant:\n%s\n", got, want)
			}
		})
	}
}
