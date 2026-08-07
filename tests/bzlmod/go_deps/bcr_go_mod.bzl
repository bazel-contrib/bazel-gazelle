"""
Unit test version of //tests/bcr/go_mod, simplified.

This exercises as much of the go_deps API as we reasonably can from a unit test.

This test case has copies of files from gazelle and tests/bcr/go_mod, but
they don't need to be kept in sync.
"""

load(
    "//internal/bzlmod:go_deps_test_case.bzl",
    "archive_override_tag",
    "config_tag",
    "from_file_tag",
    "gazelle_default_attributes_tag",
    "gazelle_override_tag",
    "module",
    "module_override_tag",
    "module_tag",
    "tags",
    "test_case",
)

TEST = test_case(
    name = "bcr_go_mod",
    modules = [
        module(
            name = "gazelle_bcr_go_mod_tests",
            is_root = True,
            tags = tags(
                config = [config_tag(
                    go_env = {
                        "GOPRIVATE": "example.com/*",
                    },
                )],
                from_file = [from_file_tag(go_mod = "//:go.mod")],
                module = [
                    module_tag(
                        path = "github.com/stretchr/testify",
                        sum = "h1:pSgiaMZlXftHpm5L7V1+rVB+AZJydKsMxsQBIJw4PKk=",
                        version = "v1.8.0",
                    ),
                    module_tag(
                        indirect = True,
                        path = "gopkg.in/yaml.v3",
                        sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
                        version = "v3.0.1",
                    ),
                    module_tag(
                        indirect = True,
                        path = "github.com/davecgh/go-spew",
                        sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
                        version = "v1.1.1",
                    ),
                ],
                gazelle_default_attributes = [gazelle_default_attributes_tag(
                    build_file_generation = "on",
                    directives = ["gazelle:proto disable"],
                )],
                gazelle_override = [
                    gazelle_override_tag(
                        directives = [
                            "gazelle:build_file_name BUILD.bazel",
                        ],
                        path = "github.com/envoyproxy/protoc-gen-validate",
                    ),
                    gazelle_override_tag(
                        directives = [
                            "gazelle:build_file_name BUILD.bazel",
                            "gazelle:build_file_proto_mode disable_global",
                        ],
                        path = "github.com/google/safetext",
                    ),
                    gazelle_override_tag(
                        directives = [
                            "gazelle:go_naming_convention go_default_library",
                        ],
                        path = "github.com/stretchr/testify",
                    ),
                    gazelle_override_tag(
                        build_file_generation = "clean",
                        path = "github.com/bazelbuild/buildtools",
                    ),
                    gazelle_override_tag(
                        build_file_generation = "clean",
                        path = "github.com/google/go-jsonnet",
                    ),
                    gazelle_override_tag(
                        build_file_generation = "off",
                        path = "example.org/proto_compat",
                    ),
                    gazelle_override_tag(
                        directives = [
                            # Verify that the build naming convention is picked up by Gazelle when it
                            # emits references to this repo.
                            "gazelle:go_naming_convention go_default_library",
                        ],
                        path = "gopkg.in/yaml.v3",
                    ),
                ],
                module_override = [
                    module_override_tag(
                        patch_cmds = [
                            "echo 'const PatchCmdHello = \"patch_cmd_hello\"' >> require/require.go",
                        ],
                        patch_strip = 1,
                        patches = [
                            "//patches:testify.patch",
                        ],
                        path = "github.com/stretchr/testify",
                    ),
                    module_override_tag(
                        path = "example.org/collision_mangle",
                        repo_name = "org_example_collision_mangle_alt",
                    ),
                ],
                archive_override = [
                    archive_override_tag(
                        patch_strip = 1,
                        patches = [
                            "//patches:buildtools.patch",
                        ],
                        path = "github.com/bazelbuild/buildtools",
                        sha256 = "05d7c3d2bd3cc0b02d15672fefa0d6be48c7aebe459c1c99dced7ac5e598508f",
                        strip_prefix = "buildtools-ae8e3206e815d086269eb208b01f300639a4b194",
                        urls = [
                            "https://github.com/bazelbuild/buildtools/archive/ae8e3206e815d086269eb208b01f300639a4b194.tar.gz",
                        ],
                    ),
                ],
            ),
        ),
        module(
            name = "gazelle",
            tags = tags(
                from_file = [
                    from_file_tag(go_work = "@@gazelle//:go.work"),
                ],
            ),
        ),
    ],
    files = {
        "/gazelle_bcr_go_mod_tests/go.mod": """
// This will stop go mod from descending into this directory.
module github.com/bazelbuild/bazel-gazelle/tests/bcr/go_mod

go 1.24.12

// Validate go.mod replace directives can be properly used:
replace github.com/bmatcuk/doublestar/v4 => github.com/bmatcuk/doublestar/v4 v4.9.1

require (
	example.org/collision/mangle v1.0.0
	example.org/collision_mangle v1.0.0
	example.org/hello v1.0.0
	example.org/proto_compat v1.0.0
	github.com/DataDog/sketches-go v1.4.1
	github.com/bazelbuild/buildtools v0.0.0-20251031164759-f48b23493530
	github.com/bazelbuild/rules_go v0.59.0
	// NOTE: keep <4.7.0 to test the 'replace'
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/cloudflare/circl v1.3.8
	github.com/envoyproxy/protoc-gen-validate v1.3.0
	github.com/fmeum/dep_on_gazelle v1.0.0
	github.com/google/go-jsonnet v0.20.0
	github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2
	github.com/stretchr/testify v1.8.0
	golang.org/x/sys v0.39.0
	google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20250929231259-57b25ae835d4
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/bazelbuild/bazel-gazelle v0.30.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.6.1 // indirect
	mvdan.cc/gofumpt v0.9.2 // indirect
	rsc.io/quote v1.5.2 // indirect
	rsc.io/sampler v1.3.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

require golang.org/x/tools/go/expect v0.1.1-deprecated // indirect

replace example.org/hello => ../../fixtures/hello

replace example.org/proto_compat => ../fixtures/proto_compat

replace example.org/collision/mangle => ../fixtures/collision_slash

replace example.org/collision_mangle => ../fixtures/collision_underscore

tool (
	honnef.co/go/tools/cmd/staticcheck
	mvdan.cc/gofumpt
)
""",
        "/gazelle_bcr_go_mod_tests/go.sum": """github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c h1:pxW6RcqyfI9/kWtOwnv/G+AzdKuy2ZrqINhenH4HyNs=
github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c/go.mod h1:ukJfTF/6rtPPRCnwkur4qwRxa8vTRFBF0uk2lLoLwho=
github.com/DataDog/sketches-go v1.4.1 h1:j5G6as+9FASM2qC36lvpvQAj9qsv/jUs3FtO8CwZNAY=
github.com/DataDog/sketches-go v1.4.1/go.mod h1:xJIXldczJyyjnbDop7ZZcLxJdV3+7Kra7H1KMgpgkLk=
github.com/bazelbuild/bazel-gazelle v0.30.0 h1:q9XLWQSCA5NZPJ98WFqicHkq6fxrDPnfvMO1XycQBMg=
github.com/bazelbuild/bazel-gazelle v0.30.0/go.mod h1:6RxhjM1v/lTpD3JlMpKUCcdus4tvdqsqdfbjYi+czYs=
github.com/bazelbuild/buildtools v0.0.0-20251031164759-f48b23493530 h1:1eLp2tJnZQrdCh+aCS/ndQdJqX2N32QcMiEL1DzDGO8=
github.com/bazelbuild/buildtools v0.0.0-20251031164759-f48b23493530/go.mod h1:PLNUetjLa77TCCziPsz0EI8a6CUxgC+1jgmWv0H25tg=
github.com/bazelbuild/rules_go v0.59.0 h1:RLhOwYIqeMgBpKelHEWTfIPjA37so3oa/rX+/qqq/P4=
github.com/bazelbuild/rules_go v0.59.0/go.mod h1:Pn30cb4M513fe2rQ6GiJ3q8QyrRsgC7zhuDvi50Lw4Y=
github.com/bmatcuk/doublestar/v4 v4.9.1 h1:X8jg9rRZmJd4yRy7ZeNDRnM+T3ZfHv15JiBJ/avrEXE=
github.com/bmatcuk/doublestar/v4 v4.9.1/go.mod h1:xBQ8jztBU6kakFMg+8WGxn0c6z1fTSPVIjEY1Wr7jzc=
github.com/cespare/xxhash/v2 v2.3.0 h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=
github.com/cespare/xxhash/v2 v2.3.0/go.mod h1:VGX0DQ3Q6kWi7AoAeZDth3/j3BFtOZR5XLFGgcrjCOs=
github.com/cloudflare/circl v1.3.8 h1:j+V8jJt09PoeMFIu2uh5JUyEaIHTXVOHslFoLNAKqwI=
github.com/cloudflare/circl v1.3.8/go.mod h1:PDRU+oXvdD7KCtgKxW95M5Z8BpSCJXQORiZFnBQS5QU=
github.com/davecgh/go-spew v1.1.0/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/envoyproxy/protoc-gen-validate v1.3.0 h1:TvGH1wof4H33rezVKWSpqKz5NXWg5VPuZ0uONDT6eb4=
github.com/envoyproxy/protoc-gen-validate v1.3.0/go.mod h1:HvYl7zwPa5mffgyeTUHA9zHIH36nmrm7oCbo4YKoSWA=
github.com/fmeum/dep_on_gazelle v1.0.0 h1:7gEtQ2CoD77tYca+1iUnKjIBUZ4mX7mZwjdWp3uuN/E=
github.com/fmeum/dep_on_gazelle v1.0.0/go.mod h1:VYCjwfsyRHOJL8oenaEjhIzgM7Oj70iTxgJ2RfXbYr0=
github.com/go-logr/logr v1.4.3 h1:CjnDlHq8ikf6E492q6eKboGOC0T8CDaOvkHCIg8idEI=
github.com/go-logr/logr v1.4.3/go.mod h1:9T104GzyrTigFIr8wt5mBrctHMim0Nb2HLGrmQ40KvY=
github.com/go-logr/stdr v1.2.2 h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=
github.com/go-logr/stdr v1.2.2/go.mod h1:mMo/vtBO5dYbehREoey6XUKy/eSumjCCveDpRre4VKE=
github.com/go-quicktest/qt v1.101.0 h1:O1K29Txy5P2OK0dGo59b7b0LR6wKfIhttaAhHUyn7eI=
github.com/go-quicktest/qt v1.101.0/go.mod h1:14Bz/f7NwaXPtdYEgzsx46kqSxVwTbzVZsDC26tQJow=
github.com/golang/protobuf v1.5.0/go.mod h1:FsONVRAS9T7sI+LIUmWTfcYkHO4aIWwzhcaSAoJOfIk=
github.com/golang/protobuf v1.5.2/go.mod h1:XVQd3VNwM+JqD3oG2Ue2ip4fOMUkwXdXDdiuN0vRsmY=
github.com/golang/protobuf v1.5.4 h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=
github.com/golang/protobuf v1.5.4/go.mod h1:lnTiLA8Wa4RWRcIUkrtSVa5nRhsEGBg48fD6rSs7xps=
github.com/google/go-cmp v0.5.5/go.mod h1:v8dTdLbMG2kIc/vJvl+f65V22dbkXbowE6jgT/gNBxE=
github.com/google/go-cmp v0.7.0 h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=
github.com/google/go-cmp v0.7.0/go.mod h1:pXiqmnSA92OHEEa9HXL2W4E7lf9JzCmGVUdgjX3N/iU=
github.com/google/go-jsonnet v0.20.0 h1:WG4TTSARuV7bSm4PMB4ohjxe33IHT5WVTrJSU33uT4g=
github.com/google/go-jsonnet v0.20.0/go.mod h1:VbgWF9JX7ztlv770x/TolZNGGFfiHEVx9G6ca2eUmeA=
github.com/google/gofuzz v1.2.0 h1:xRy4A+RhZaiKjJ1bPfwQ8sedCA+YS2YcCHW6ec7JMi0=
github.com/google/gofuzz v1.2.0/go.mod h1:dBl0BpW6vV/+mYPU4Po3pmUjxk6FQPldtuIdl/M65Eg=
github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2 h1:SJ+NtwL6QaZ21U+IrK7d0gGgpjGGvd2kz+FzTHVzdqI=
github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2/go.mod h1:Tv1PlzqC9t8wNnpPdctvtSUOPUUg4SHeE6vR1Ir2hmg=
github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=
github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/kr/pretty v0.3.1 h1:flRD4NNwYAUpkphVc1HcthR4KEIFJ65n8Mw5qdRn3LE=
github.com/kr/pretty v0.3.1/go.mod h1:hoEshYVHaxMs3cyo3Yncou5ZscifuDolrwPKZanG3xk=
github.com/kr/text v0.2.0 h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=
github.com/kr/text v0.2.0/go.mod h1:eLer722TekiGuMkidMxC/pM04lWEeraHUUmBw8l2grE=
github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e h1:fD57ERR4JtEqsWbfPhv4DMiApHyliiK5xCTNVSPiaAs=
github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e/go.mod h1:zD1mROLANZcx1PVRCS0qkT7pwLkGfwJo4zjcN/Tysno=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
github.com/rogpeppe/go-internal v1.14.1 h1:UQB4HGPB6osV0SQTLymcB4TgvyWu6ZyliaW0tI/otEQ=
github.com/rogpeppe/go-internal v1.14.1/go.mod h1:MaRKkUm5W0goXpeCfT7UZI6fk/L7L7so1lCWt35ZSgc=
github.com/sergi/go-diff v1.1.0 h1:we8PVUC3FE2uYfodKH/nBHMSetSfHDR6scGdBi+erh0=
github.com/sergi/go-diff v1.1.0/go.mod h1:STckp+ISIX8hZLjrqAeVduY0gWCT9IjLuqbuNXdaHfM=
github.com/stretchr/objx v0.1.0/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
github.com/stretchr/objx v0.4.0/go.mod h1:YvHI0jy2hoMjB+UWwv71VJQ9isScKT/TqJzVSSt89Yw=
github.com/stretchr/testify v1.6.1/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.7.1/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.8.0 h1:pSgiaMZlXftHpm5L7V1+rVB+AZJydKsMxsQBIJw4PKk=
github.com/stretchr/testify v1.8.0/go.mod h1:yNjHg4UonilssWZ8iaSj1OCr/vHnekPRkoO+kdMU+MU=
go.opentelemetry.io/auto/sdk v1.2.1 h1:jXsnJ4Lmnqd11kwkBV2LgLoFMZKizbCi5fNZ/ipaZ64=
go.opentelemetry.io/auto/sdk v1.2.1/go.mod h1:KRTj+aOaElaLi+wW1kO/DZRXwkF4C5xPbEe3ZiIhN7Y=
go.opentelemetry.io/otel v1.39.0 h1:8yPrr/S0ND9QEfTfdP9V+SiwT4E0G7Y5MO7p85nis48=
go.opentelemetry.io/otel v1.39.0/go.mod h1:kLlFTywNWrFyEdH0oj2xK0bFYZtHRYUdv1NklR/tgc8=
go.opentelemetry.io/otel/metric v1.39.0 h1:d1UzonvEZriVfpNKEVmHXbdf909uGTOQjA0HF0Ls5Q0=
go.opentelemetry.io/otel/metric v1.39.0/go.mod h1:jrZSWL33sD7bBxg1xjrqyDjnuzTUB0x1nBERXd7Ftcs=
go.opentelemetry.io/otel/sdk v1.39.0 h1:nMLYcjVsvdui1B/4FRkwjzoRVsMK8uL/cj0OyhKzt18=
go.opentelemetry.io/otel/sdk v1.39.0/go.mod h1:vDojkC4/jsTJsE+kh+LXYQlbL8CgrEcwmt1ENZszdJE=
go.opentelemetry.io/otel/sdk/metric v1.39.0 h1:cXMVVFVgsIf2YL6QkRF4Urbr/aMInf+2WKg+sEJTtB8=
go.opentelemetry.io/otel/sdk/metric v1.39.0/go.mod h1:xq9HEVH7qeX69/JnwEfp6fVq5wosJsY1mt4lLfYdVew=
go.opentelemetry.io/otel/trace v1.39.0 h1:2d2vfpEDmCJ5zVYz7ijaJdOF59xLomrvj7bjt6/qCJI=
go.opentelemetry.io/otel/trace v1.39.0/go.mod h1:88w4/PnZSazkGzz/w84VHpQafiU4EtqqlVdxWy+rNOA=
golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 h1:1P7xPZEwZMoBoz0Yze5Nx2/4pxj6nw9ZqHWXqP0iRgQ=
golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678/go.mod h1:AbB0pIl9nAr9wVwH+Z2ZpaocVmF5I4GyWCDIsVjR0bk=
golang.org/x/mod v0.30.0 h1:fDEXFVZ/fmCKProc/yAXXUijritrDzahmwwefnjoPFk=
golang.org/x/mod v0.30.0/go.mod h1:lAsf5O2EvJeSFMiBxXDki7sCgAxEUcZHXoXMKT4GJKc=
golang.org/x/net v0.48.0 h1:zyQRTTrjc33Lhh0fBgT/H3oZq9WuvRR5gPC70xpDiQU=
golang.org/x/net v0.48.0/go.mod h1:+ndRgGjkh8FGtu1w1FGbEC31if4VrNVMuKTgcAAnQRY=
golang.org/x/sync v0.19.0 h1:vV+1eWNmZ5geRlYjzm2adRgW2/mcpevXNg50YZtPCE4=
golang.org/x/sync v0.19.0/go.mod h1:9KTHXmSnoGruLpwFjVSX0lNNA75CykiMECbovNTZqGI=
golang.org/x/sys v0.39.0 h1:CvCKL8MeisomCi6qNZ+wbb0DN9E5AATixKsvNtMoMFk=
golang.org/x/sys v0.39.0/go.mod h1:OgkHotnGiDImocRcuBABYBEXf8A9a87e/uXjp9XT3ks=
golang.org/x/text v0.0.0-20170915032832-14c0d48ead0c/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
golang.org/x/text v0.32.0 h1:ZD01bjUt1FQ9WJ0ClOL5vxgxOI/sVCNgX1YtKwcY0mU=
golang.org/x/text v0.32.0/go.mod h1:o/rUWzghvpD5TXrTIBuJU77MTaN0ljMWE47kxGJQ7jY=
golang.org/x/tools v0.39.0 h1:ik4ho21kwuQln40uelmciQPp9SipgNDdrafrYA4TmQQ=
golang.org/x/tools v0.39.0/go.mod h1:JnefbkDPyD8UU2kI5fuf8ZX4/yUeh9W877ZeBONxUqQ=
golang.org/x/tools/go/expect v0.1.1-deprecated h1:jpBZDwmgPhXsKZC6WhL20P4b/wmnpsEAGHaNy0n/rJM=
golang.org/x/tools/go/expect v0.1.1-deprecated/go.mod h1:eihoPOH+FgIqa3FpoTwguz/bVUSGBlGQU67vpBeOrBY=
golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
gonum.org/v1/gonum v0.16.0 h1:5+ul4Swaf3ESvrOnidPp4GZbzf0mxVQpDCYUQE7OJfk=
gonum.org/v1/gonum v0.16.0/go.mod h1:fef3am4MQ93R2HHpKnLk4/Tbh/s0+wqD5nfa6Pnwy4E=
google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f h1:387Y+JbxF52bmesc8kq1NyYIp33dnxCw6eiA7JMsTmw=
google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f/go.mod h1:0joYwWwLQh18AOj8zMYeZLjzuqcYTU3/nC5JdCvC3JI=
google.golang.org/genproto/googleapis/bytestream v0.0.0-20250929231259-57b25ae835d4 h1:D5BwyPCIfMlKQkiIkAlGFm/YSPlj4kMamJDgLshytz4=
google.golang.org/genproto/googleapis/bytestream v0.0.0-20250929231259-57b25ae835d4/go.mod h1:YUQUKndxDbAanQC0ln4pZ3Sis3N5sqgDte2XQqufkJc=
google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 h1:gRkg/vSppuSQoDjxyiGfN4Upv/h/DQmIR10ZU8dh4Ww=
google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217/go.mod h1:7i2o+ce6H/6BluujYR+kqX3GKH+dChPTQU19wjRPiGk=
google.golang.org/grpc v1.79.3 h1:sybAEdRIEtvcD68Gx7dmnwjZKlyfuc61Dyo9pGXXkKE=
google.golang.org/grpc v1.79.3/go.mod h1:KmT0Kjez+0dde/v2j9vzwoAScgEPx/Bw1CYChhHLrHQ=
google.golang.org/protobuf v1.26.0-rc.1/go.mod h1:jlhhOSvTdKEhbULTjvd4ARK9grFBp09yW+WbY/TyQbw=
google.golang.org/protobuf v1.26.0/go.mod h1:9q0QmTI4eRPtz6boOQmLYwt+qCgq0jsYwAQnmE0givc=
google.golang.org/protobuf v1.28.0/go.mod h1:HV8QOd/L58Z+nl8r43ehVNZIU/HEI6OcFqwMG9pJV4I=
google.golang.org/protobuf v1.36.10 h1:AYd7cD/uASjIL6Q9LiTjz8JLcrh/88q5UObnmY3aOOE=
google.golang.org/protobuf v1.36.10/go.mod h1:HTf+CrKn2C3g5S8VImy6tdcUvCska2kB7j23XfzDpco=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b h1:QRR6H1YWRnHb4Y/HeNFCTJLFVxaq6wH4YuVdsUOr75U=
gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/yaml.v2 v2.2.7 h1:VUgggvou5XRW9mHwD/yXxIYSMtY0zoKQf/v226p2nyo=
gopkg.in/yaml.v2 v2.2.7/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
honnef.co/go/tools v0.6.1 h1:R094WgE8K4JirYjBaOpz/AvTyUu/3wbmAoskKN/pxTI=
honnef.co/go/tools v0.6.1/go.mod h1:3puzxxljPCe8RGJX7BIy1plGbxEOZni5mR2aXe3/uk4=
mvdan.cc/gofumpt v0.9.2 h1:zsEMWL8SVKGHNztrx6uZrXdp7AX8r421Vvp23sz7ik4=
mvdan.cc/gofumpt v0.9.2/go.mod h1:iB7Hn+ai8lPvofHd9ZFGVg2GOr8sBUw1QUWjNbmIL/s=
rsc.io/quote v1.5.2 h1:w5fcysjrx7yqtD/aO+QwRjYZOKnaM9Uh2b40tElTs3Y=
rsc.io/quote v1.5.2/go.mod h1:LzX7hefJvL54yjefDEDHNONDjII0t9xZLPXsUe+TKr0=
rsc.io/sampler v1.3.0 h1:7uVkIFmeBqHfdjD+gZwtXXI+RODJ2Wc4O7MPEh/QiW4=
rsc.io/sampler v1.3.0/go.mod h1:T1hPZKmBbMNahiBKFy5HrXp6adAjACjK9JXDnKaTXpA=
sigs.k8s.io/yaml v1.1.0 h1:4A07+ZFc2wgJwo8YNlQpr1rVlgUDlxXHhPJciaPY5gs=
sigs.k8s.io/yaml v1.1.0/go.mod h1:UJmg0vDUVViEyp3mgSv9WPwZCDxu4rQW1olrI1uml+o=
""",
        "/gazelle/go.work": """
go 1.24.12

use (
	.
	./v2
)
""",
        "/gazelle/go.mod": """
module github.com/bazelbuild/bazel-gazelle

go 1.24.12

require (
	github.com/bazel-contrib/bazel-gazelle/v2 v2.0.0-2
	github.com/bazelbuild/buildtools v0.0.0-20250930140053-2eb4fccefb52
	github.com/bazelbuild/rules_go v0.59.0
	github.com/bmatcuk/doublestar/v4 v4.9.1
	github.com/fsnotify/fsnotify v1.7.0
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.6.0
	github.com/pmezard/go-difflib v1.0.0
	golang.org/x/mod v0.23.0
	golang.org/x/sync v0.11.0
	golang.org/x/tools/go/vcs v0.1.0-deprecated
	google.golang.org/protobuf v1.36.3
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
	google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/grpc v1.67.3 // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1 // indirect
)
""",
        "/gazelle/go.sum": """
github.com/bazel-contrib/bazel-gazelle/v2 v2.0.0-1 h1:Aney60tyjQn3ON+KPFQhEZ7saU1EtOhlCrGTKGkxiEI=
github.com/bazel-contrib/bazel-gazelle/v2 v2.0.0-1/go.mod h1:B76PjnaM2KyUG8ypV/ePHHDmGbyUNZ2PCEdEQIJISQM=
github.com/bazel-contrib/bazel-gazelle/v2 v2.0.0-2 h1:lSINS7DK4xjm0Z4CBd7onO63uIe6WUsEZ/0voExa+JY=
github.com/bazel-contrib/bazel-gazelle/v2 v2.0.0-2/go.mod h1:B76PjnaM2KyUG8ypV/ePHHDmGbyUNZ2PCEdEQIJISQM=
github.com/bazelbuild/buildtools v0.0.0-20250930140053-2eb4fccefb52 h1:njQAmjTv/YHRm/0Lfv9DXHFZ4MdT2IA/RKHTnqZkgDw=
github.com/bazelbuild/buildtools v0.0.0-20250930140053-2eb4fccefb52/go.mod h1:PLNUetjLa77TCCziPsz0EI8a6CUxgC+1jgmWv0H25tg=
github.com/bazelbuild/rules_go v0.53.0 h1:u160DT+RRb+Xb2aSO4piN8xhs4aZvWz2UDXCq48F4ao=
github.com/bazelbuild/rules_go v0.53.0/go.mod h1:xB1jfsYHWlnZyPPxzlOSst4q2ZAwS251Mp9Iw6TPuBc=
github.com/bazelbuild/rules_go v0.59.0 h1:RLhOwYIqeMgBpKelHEWTfIPjA37so3oa/rX+/qqq/P4=
github.com/bazelbuild/rules_go v0.59.0/go.mod h1:Pn30cb4M513fe2rQ6GiJ3q8QyrRsgC7zhuDvi50Lw4Y=
github.com/bmatcuk/doublestar/v4 v4.9.1 h1:X8jg9rRZmJd4yRy7ZeNDRnM+T3ZfHv15JiBJ/avrEXE=
github.com/bmatcuk/doublestar/v4 v4.9.1/go.mod h1:xBQ8jztBU6kakFMg+8WGxn0c6z1fTSPVIjEY1Wr7jzc=
github.com/fsnotify/fsnotify v1.7.0 h1:8JEhPFa5W2WU7YfeZzPNqzMP6Lwt7L2715Ggo0nosvA=
github.com/fsnotify/fsnotify v1.7.0/go.mod h1:40Bi/Hjc2AVfZrqy+aj+yEI+/bRxZnMJyTJwOpGvigM=
github.com/gogo/protobuf v1.3.2 h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=
github.com/gogo/protobuf v1.3.2/go.mod h1:P1XiOD3dCwIKUDQYPy72D8LYyHL2YPYrpS2s69NZV8Q=
github.com/golang/mock v1.7.0-rc.1 h1:YojYx61/OLFsiv6Rw1Z96LpldJIy31o+UHmwAUMJ6/U=
github.com/golang/mock v1.7.0-rc.1/go.mod h1:s42URUywIqd+OcERslBJvOjepvNymP31m3q8d/GkuRs=
github.com/golang/protobuf v1.5.4 h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=
github.com/golang/protobuf v1.5.4/go.mod h1:lnTiLA8Wa4RWRcIUkrtSVa5nRhsEGBg48fD6rSs7xps=
github.com/google/go-cmp v0.6.0 h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI=
github.com/google/go-cmp v0.6.0/go.mod h1:17dUlkBOakJ0+DkrSSNjCkIjxS6bF9zb3elmeNGIjoY=
github.com/kisielk/errcheck v1.5.0/go.mod h1:pFxgyoBC7bSaBwPgfKdkLd5X25qrDl4LWUI2bnpBCr8=
github.com/kisielk/gotool v1.0.0/go.mod h1:XhKaO+MFFWcvkIS/tQcRk01m1F5IRFswLeQ+oQHNcck=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
github.com/yuin/goldmark v1.1.27/go.mod h1:3hX8gzYuyVAZsxl0MRgGTJEmQBFcNTphYh9decYSb74=
github.com/yuin/goldmark v1.2.1/go.mod h1:3hX8gzYuyVAZsxl0MRgGTJEmQBFcNTphYh9decYSb74=
github.com/yuin/goldmark v1.4.1/go.mod h1:mwnBkeHKe2W/ZEtQ+71ViKU8L12m81fl3OWwC1Zlc8k=
golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2/go.mod h1:djNgcEr1/C05ACkg1iLfiJU5Ep61QUkGW8qpdssI0+w=
golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550/go.mod h1:yigFU9vqHzYiE8UmvKecakEJjdnWj3jj499lnFckfCI=
golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9/go.mod h1:LzIPMQfyMNhhGPhUkYOs5KpL4U8rLKemX1yGLhDgUto=
golang.org/x/mod v0.2.0/go.mod h1:s0Qsj1ACt9ePp/hMypM3fl4fZqREWJwdYDEqhRiZZUA=
golang.org/x/mod v0.3.0/go.mod h1:s0Qsj1ACt9ePp/hMypM3fl4fZqREWJwdYDEqhRiZZUA=
golang.org/x/mod v0.5.1/go.mod h1:5OXOZSfqPIIbmVBIIKWRFfZjPR0E5r58TLhUjH0a2Ro=
golang.org/x/mod v0.22.0 h1:D4nJWe9zXqHOmWqj4VMOJhvzj7bEZg4wEYa759z1pH4=
golang.org/x/mod v0.22.0/go.mod h1:6SkKJ3Xj0I0BrPOZoBy3bdMptDDU9oJrpohJ3eWZ1fY=
golang.org/x/mod v0.23.0 h1:Zb7khfcRGKk+kqfxFaP5tZqCnDZMjC5VtUBs87Hr6QM=
golang.org/x/mod v0.23.0/go.mod h1:6SkKJ3Xj0I0BrPOZoBy3bdMptDDU9oJrpohJ3eWZ1fY=
golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
golang.org/x/net v0.0.0-20190620200207-3b0461eec859/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
golang.org/x/net v0.0.0-20200226121028-0de0cce0169b/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
golang.org/x/net v0.0.0-20201021035429-f5854403a974/go.mod h1:sp8m0HH+o8qH0wwXwYZr8TS3Oi6o0r6Gce1SSxlDquU=
golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f/go.mod h1:9nx3DQGgdP8bBQD5qxJ1jj9UTztislL4KSBs9R2vV5Y=
golang.org/x/net v0.35.0 h1:T5GQRQb2y08kTAByq9L4/bz8cipCdA8FbRTXewonqY8=
golang.org/x/net v0.35.0/go.mod h1:EglIi67kWsHKlRzzVMUD93VMSWGFOMSZgxFjparz1Qk=
golang.org/x/sync v0.0.0-20190423024810-112230192c58/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20210220032951-036812b2e83c/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.10.0 h1:3NQrjDixjgGwUOCaF8w2+VYHv0Ve/vGYSbdkTa98gmQ=
golang.org/x/sync v0.10.0/go.mod h1:Czt+wKu1gCyEFDUtn0jG5QVvpJ6rzVqr5aXyt9drQfk=
golang.org/x/sync v0.11.0 h1:GGz8+XQP4FvTTrjZPzNKTMFtSXH80RAzG+5ghFPgK9w=
golang.org/x/sync v0.11.0/go.mod h1:Czt+wKu1gCyEFDUtn0jG5QVvpJ6rzVqr5aXyt9drQfk=
golang.org/x/sys v0.0.0-20190215142949-d0b11bdaac8a/go.mod h1:STP8DvDyc/dI5b8T5hshtkjS+E42TnysNCUPdjciGhY=
golang.org/x/sys v0.0.0-20190412213103-97732733099d/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20201119102817-f84b799fce68/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20210423082822-04245dca01da/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20211019181941-9d821ace8654/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.28.0 h1:Fksou7UEQUWlKvIdsqzJmUmCX3cZuD2+P3XyyzwMhlA=
golang.org/x/sys v0.28.0/go.mod h1:/VUhepiaJMQUp4+oa/7Zr1D23ma6VTLIYjOOTFZPUcA=
golang.org/x/sys v0.30.0 h1:QjkSwP/36a20jFYWkSue1YwXzLmsV5Gfq7Eiy72C1uc=
golang.org/x/sys v0.30.0/go.mod h1:/VUhepiaJMQUp4+oa/7Zr1D23ma6VTLIYjOOTFZPUcA=
golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1/go.mod h1:bj7SfCRtBDWHUb9snDiAeCFNEtKQo2Wmx5Cou7ajbmo=
golang.org/x/text v0.3.0/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
golang.org/x/text v0.3.3/go.mod h1:5Zoc/QRtKVWzQhOtBMvqHzDpF6irO9z98xDceosuGiQ=
golang.org/x/text v0.3.6/go.mod h1:5Zoc/QRtKVWzQhOtBMvqHzDpF6irO9z98xDceosuGiQ=
golang.org/x/text v0.3.7/go.mod h1:u+2+/6zg+i71rQMx5EYifcz6MCKuco9NR6JIITiCfzQ=
golang.org/x/text v0.22.0 h1:bofq7m3/HAFvbF51jz3Q9wLg3jkvSPuiZu/pD1XwgtM=
golang.org/x/text v0.22.0/go.mod h1:YRoo4H8PVmsu+E3Ou7cqLVH8oXWIHVoX0jqUWALQhfY=
golang.org/x/tools v0.0.0-20180917221912-90fa682c2a6e/go.mod h1:n7NCudcB/nEzxVGmLbDWY5pfWTLqBcC2KZ6jyYvM4mQ=
golang.org/x/tools v0.0.0-20191119224855-298f0cb1881e/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
golang.org/x/tools v0.0.0-20200619180055-7c47624df98f/go.mod h1:EkVYQZoAsY45+roYkvgYkIh4xh/qjgUK9TdY2XT94GE=
golang.org/x/tools v0.0.0-20210106214847-113979e3529a/go.mod h1:emZCQorbCU4vsT4fOWvOPXz4eW1wZW4PmDk9uLelYpA=
golang.org/x/tools v0.1.8/go.mod h1:nABZi5QlRsZVlzPpHl034qft6wpY4eDcsTt5AaioBiU=
golang.org/x/tools v0.27.0 h1:qEKojBykQkQ4EynWy4S8Weg69NumxKdn40Fce3uc/8o=
golang.org/x/tools v0.27.0/go.mod h1:sUi0ZgbwW9ZPAq26Ekut+weQPR5eIM6GQLQ1Yjm1H0Q=
golang.org/x/tools v0.30.0 h1:BgcpHewrV5AUp2G9MebG4XPFI1E2W41zU1SaqVA9vJY=
golang.org/x/tools v0.30.0/go.mod h1:c347cR/OJfw5TI+GfX7RUPNMdDRRbjvYTS0jPyvsVtY=
golang.org/x/tools/go/vcs v0.1.0-deprecated h1:cOIJqWBl99H1dH5LWizPa+0ImeeJq3t3cJjaeOWUAL4=
golang.org/x/tools/go/vcs v0.1.0-deprecated/go.mod h1:zUrvATBAvEI9535oC0yWYsLsHIV4Z7g63sNPVMtuBy8=
golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f h1:387Y+JbxF52bmesc8kq1NyYIp33dnxCw6eiA7JMsTmw=
google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f/go.mod h1:0joYwWwLQh18AOj8zMYeZLjzuqcYTU3/nC5JdCvC3JI=
google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422 h1:3UsHvIr4Wc2aW4brOaSCmcxh9ksica6fHEr8P1XhkYw=
google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422/go.mod h1:3ENsm/5D1mzDyhpzeRi1NR784I0BcofWBoSc5QqqMK4=
google.golang.org/grpc v1.67.3 h1:OgPcDAFKHnH8X3O4WcO4XUc8GRDeKsKReqbQtiCj7N8=
google.golang.org/grpc v1.67.3/go.mod h1:YGaHCc6Oap+FzBJTZLBzkGSYt/cvGPFTPxkn7QfSU8s=
google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1 h1:F29+wU6Ee6qgu9TddPgooOdaqsxTMunOoj8KA5yuS5A=
google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1/go.mod h1:5KF+wpkbTSbGcR9zteSqZV6fqFOWBl4Yde8En8MryZA=
google.golang.org/protobuf v1.36.3 h1:82DV7MYdb8anAVi3qge1wSnMDrnKK7ebr+I0hHRN1BU=
google.golang.org/protobuf v1.36.3/go.mod h1:9fA7Ob0pmnwhb644+1+CVWFRbNajQ6iRojtC/QF5bRE=
""",
        "/gazelle/v2/go.mod": """
module github.com/bazel-contrib/bazel-gazelle/v2

go 1.24.12
""",
    },
    want = [
        struct(
            name = "com_github_stretchr_testify",
            importpath = "github.com/stretchr/testify",
            build_directives = ["gazelle:go_naming_convention go_default_library"],
            patches = ["//patches:testify.patch"],
            patch_args = ["-p1"],
            patch_cmds = ["echo 'const PatchCmdHello = \"patch_cmd_hello\"' >> require/require.go"],
            version = "v1.8.0",
            sum = "h1:pSgiaMZlXftHpm5L7V1+rVB+AZJydKsMxsQBIJw4PKk=",
        ),
        struct(
            name = "in_gopkg_yaml_v3",
            importpath = "gopkg.in/yaml.v3",
            build_directives = ["gazelle:go_naming_convention go_default_library"],
            version = "v3.0.1",
            sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        ),
        struct(
            name = "com_github_davecgh_go_spew",
            importpath = "github.com/davecgh/go-spew",
            build_directives = ["gazelle:proto disable"],
            version = "v1.1.1",
            sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
        ),
        struct(
            name = "com_github_datadog_sketches_go",
            importpath = "github.com/DataDog/sketches-go",
            build_directives = ["gazelle:proto disable"],
            version = "v1.4.1",
            sum = "h1:j5G6as+9FASM2qC36lvpvQAj9qsv/jUs3FtO8CwZNAY=",
        ),
        struct(
            name = "com_github_bazelbuild_buildtools",
            importpath = "github.com/bazelbuild/buildtools",
            build_file_generation = "clean",
            patches = ["//patches:buildtools.patch"],
            patch_args = ["-p1"],
            urls = ["https://github.com/bazelbuild/buildtools/archive/ae8e3206e815d086269eb208b01f300639a4b194.tar.gz"],
            strip_prefix = "buildtools-ae8e3206e815d086269eb208b01f300639a4b194",
            sha256 = "05d7c3d2bd3cc0b02d15672fefa0d6be48c7aebe459c1c99dced7ac5e598508f",
        ),
        struct(
            name = "com_github_bmatcuk_doublestar_v4",
            importpath = "github.com/bmatcuk/doublestar/v4",
            build_directives = ["gazelle:proto disable"],
            replace = "github.com/bmatcuk/doublestar/v4",
            version = "v4.9.1",
            sum = "h1:X8jg9rRZmJd4yRy7ZeNDRnM+T3ZfHv15JiBJ/avrEXE=",
        ),
        struct(
            name = "com_github_envoyproxy_protoc_gen_validate",
            importpath = "github.com/envoyproxy/protoc-gen-validate",
            build_directives = ["gazelle:build_file_name BUILD.bazel"],
            version = "v1.3.0",
            sum = "h1:TvGH1wof4H33rezVKWSpqKz5NXWg5VPuZ0uONDT6eb4=",
        ),
        struct(
            name = "com_github_fmeum_dep_on_gazelle",
            importpath = "github.com/fmeum/dep_on_gazelle",
            build_directives = ["gazelle:proto disable"],
            version = "v1.0.0",
            sum = "h1:7gEtQ2CoD77tYca+1iUnKjIBUZ4mX7mZwjdWp3uuN/E=",
        ),
        struct(
            name = "com_github_google_go_jsonnet",
            importpath = "github.com/google/go-jsonnet",
            build_file_generation = "clean",
            version = "v0.20.0",
            sum = "h1:WG4TTSARuV7bSm4PMB4ohjxe33IHT5WVTrJSU33uT4g=",
        ),
        struct(
            name = "com_github_google_safetext",
            importpath = "github.com/google/safetext",
            build_directives = ["gazelle:build_file_name BUILD.bazel", "gazelle:build_file_proto_mode disable_global"],
            version = "v0.0.0-20220905092116-b49f7bc46da2",
            sum = "h1:SJ+NtwL6QaZ21U+IrK7d0gGgpjGGvd2kz+FzTHVzdqI=",
        ),
        struct(
            name = "org_golang_x_sys",
            importpath = "golang.org/x/sys",
            build_directives = ["gazelle:proto disable"],
            version = "v0.39.0",
            sum = "h1:CvCKL8MeisomCi6qNZ+wbb0DN9E5AATixKsvNtMoMFk=",
        ),
        struct(
            name = "org_golang_google_genproto",
            importpath = "google.golang.org/genproto",
            build_directives = ["gazelle:proto disable"],
            version = "v0.0.0-20250115164207-1a7da9e5054f",
            sum = "h1:387Y+JbxF52bmesc8kq1NyYIp33dnxCw6eiA7JMsTmw=",
        ),
        struct(
            name = "org_golang_google_genproto_googleapis_bytestream",
            importpath = "google.golang.org/genproto/googleapis/bytestream",
            build_directives = ["gazelle:proto disable"],
            version = "v0.0.0-20250929231259-57b25ae835d4",
            sum = "h1:D5BwyPCIfMlKQkiIkAlGFm/YSPlj4kMamJDgLshytz4=",
        ),
        struct(
            name = "org_golang_google_protobuf",
            importpath = "google.golang.org/protobuf",
            build_directives = ["gazelle:proto disable"],
            version = "v1.36.10",
            sum = "h1:AYd7cD/uASjIL6Q9LiTjz8JLcrh/88q5UObnmY3aOOE=",
        ),
        struct(
            name = "com_github_burntsushi_toml",
            importpath = "github.com/BurntSushi/toml",
            build_directives = ["gazelle:proto disable"],
            version = "v1.4.1-0.20240526193622-a339e1f7089c",
            sum = "h1:pxW6RcqyfI9/kWtOwnv/G+AzdKuy2ZrqINhenH4HyNs=",
        ),
        struct(
            name = "com_github_google_go_cmp",
            importpath = "github.com/google/go-cmp",
            build_directives = ["gazelle:proto disable"],
            version = "v0.7.0",
            sum = "h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=",
        ),
        struct(
            name = "com_github_pmezard_go_difflib",
            importpath = "github.com/pmezard/go-difflib",
            build_directives = ["gazelle:proto disable"],
            version = "v1.0.0",
            sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
        ),
        struct(
            name = "org_golang_x_exp_typeparams",
            importpath = "golang.org/x/exp/typeparams",
            build_directives = ["gazelle:proto disable"],
            version = "v0.0.0-20231108232855-2478ac86f678",
            sum = "h1:1P7xPZEwZMoBoz0Yze5Nx2/4pxj6nw9ZqHWXqP0iRgQ=",
        ),
        struct(
            name = "org_golang_x_mod",
            importpath = "golang.org/x/mod",
            build_directives = ["gazelle:proto disable"],
            version = "v0.30.0",
            sum = "h1:fDEXFVZ/fmCKProc/yAXXUijritrDzahmwwefnjoPFk=",
        ),
        struct(
            name = "org_golang_x_net",
            importpath = "golang.org/x/net",
            build_directives = ["gazelle:proto disable"],
            version = "v0.48.0",
            sum = "h1:zyQRTTrjc33Lhh0fBgT/H3oZq9WuvRR5gPC70xpDiQU=",
        ),
        struct(
            name = "org_golang_x_sync",
            importpath = "golang.org/x/sync",
            build_directives = ["gazelle:proto disable"],
            version = "v0.19.0",
            sum = "h1:vV+1eWNmZ5geRlYjzm2adRgW2/mcpevXNg50YZtPCE4=",
        ),
        struct(
            name = "org_golang_x_text",
            importpath = "golang.org/x/text",
            build_directives = ["gazelle:proto disable"],
            version = "v0.32.0",
            sum = "h1:ZD01bjUt1FQ9WJ0ClOL5vxgxOI/sVCNgX1YtKwcY0mU=",
        ),
        struct(
            name = "org_golang_x_tools",
            importpath = "golang.org/x/tools",
            build_directives = ["gazelle:proto disable"],
            version = "v0.39.0",
            sum = "h1:ik4ho21kwuQln40uelmciQPp9SipgNDdrafrYA4TmQQ=",
        ),
        struct(
            name = "org_golang_google_genproto_googleapis_rpc",
            importpath = "google.golang.org/genproto/googleapis/rpc",
            build_directives = ["gazelle:proto disable"],
            version = "v0.0.0-20251202230838-ff82c1b0f217",
            sum = "h1:gRkg/vSppuSQoDjxyiGfN4Upv/h/DQmIR10ZU8dh4Ww=",
        ),
        struct(
            name = "org_golang_google_grpc",
            importpath = "google.golang.org/grpc",
            build_directives = ["gazelle:proto disable"],
            version = "v1.79.3",
            sum = "h1:sybAEdRIEtvcD68Gx7dmnwjZKlyfuc61Dyo9pGXXkKE=",
        ),
        struct(
            name = "in_gopkg_yaml_v2",
            importpath = "gopkg.in/yaml.v2",
            build_directives = ["gazelle:proto disable"],
            version = "v2.2.7",
            sum = "h1:VUgggvou5XRW9mHwD/yXxIYSMtY0zoKQf/v226p2nyo=",
        ),
        struct(
            name = "co_honnef_go_tools",
            importpath = "honnef.co/go/tools",
            build_directives = ["gazelle:proto disable"],
            version = "v0.6.1",
            sum = "h1:R094WgE8K4JirYjBaOpz/AvTyUu/3wbmAoskKN/pxTI=",
        ),
        struct(
            name = "cc_mvdan_gofumpt",
            importpath = "mvdan.cc/gofumpt",
            build_directives = ["gazelle:proto disable"],
            version = "v0.9.2",
            sum = "h1:zsEMWL8SVKGHNztrx6uZrXdp7AX8r421Vvp23sz7ik4=",
        ),
        struct(
            name = "io_rsc_quote",
            importpath = "rsc.io/quote",
            build_directives = ["gazelle:proto disable"],
            version = "v1.5.2",
            sum = "h1:w5fcysjrx7yqtD/aO+QwRjYZOKnaM9Uh2b40tElTs3Y=",
        ),
        struct(
            name = "io_rsc_sampler",
            importpath = "rsc.io/sampler",
            build_directives = ["gazelle:proto disable"],
            version = "v1.3.0",
            sum = "h1:7uVkIFmeBqHfdjD+gZwtXXI+RODJ2Wc4O7MPEh/QiW4=",
        ),
        struct(
            name = "io_k8s_sigs_yaml",
            importpath = "sigs.k8s.io/yaml",
            build_directives = ["gazelle:proto disable"],
            version = "v1.1.0",
            sum = "h1:4A07+ZFc2wgJwo8YNlQpr1rVlgUDlxXHhPJciaPY5gs=",
        ),
        struct(
            name = "org_golang_x_tools_go_expect",
            importpath = "golang.org/x/tools/go/expect",
            build_directives = ["gazelle:proto disable"],
            version = "v0.1.1-deprecated",
            sum = "h1:jpBZDwmgPhXsKZC6WhL20P4b/wmnpsEAGHaNy0n/rJM=",
        ),
        struct(
            name = "com_github_fsnotify_fsnotify",
            importpath = "github.com/fsnotify/fsnotify",
            build_directives = ["gazelle:proto disable"],
            version = "v1.7.0",
            sum = "h1:8JEhPFa5W2WU7YfeZzPNqzMP6Lwt7L2715Ggo0nosvA=",
        ),
        struct(
            name = "com_github_golang_protobuf",
            importpath = "github.com/golang/protobuf",
            build_directives = ["gazelle:proto disable"],
            version = "v1.5.4",
            sum = "h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=",
        ),
        struct(
            name = "org_golang_x_tools_go_vcs",
            importpath = "golang.org/x/tools/go/vcs",
            build_directives = ["gazelle:proto disable"],
            version = "v0.1.0-deprecated",
            sum = "h1:cOIJqWBl99H1dH5LWizPa+0ImeeJq3t3cJjaeOWUAL4=",
        ),
        struct(
            name = "com_github_gogo_protobuf",
            importpath = "github.com/gogo/protobuf",
            build_directives = ["gazelle:proto disable"],
            version = "v1.3.2",
            sum = "h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=",
        ),
        struct(
            name = "com_github_golang_mock",
            importpath = "github.com/golang/mock",
            build_directives = ["gazelle:proto disable"],
            version = "v1.7.0-rc.1",
            sum = "h1:YojYx61/OLFsiv6Rw1Z96LpldJIy31o+UHmwAUMJ6/U=",
        ),
        struct(
            name = "org_golang_google_grpc_cmd_protoc_gen_go_grpc",
            importpath = "google.golang.org/grpc/cmd/protoc-gen-go-grpc",
            build_directives = ["gazelle:proto disable"],
            version = "v1.5.1",
            sum = "h1:F29+wU6Ee6qgu9TddPgooOdaqsxTMunOoj8KA5yuS5A=",
        ),
    ],
)
