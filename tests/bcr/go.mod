// This will stop go mod from descending into this directory.
module github.com/bazelbuild/bazel-gazelle/tests/bcr

go 1.19

// Validate go.mod replace directives can be properly used:
replace github.com/bmatcuk/doublestar/v4 => github.com/bmatcuk/doublestar v1.3.4

require (
	github.com/DataDog/sketches-go v1.4.1
	github.com/bazelbuild/buildtools v0.0.0-20230317132445-9c3c1fc0106e
	github.com/bazelbuild/rules_go v0.39.1
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/cloudflare/circl v1.3.7
	github.com/envoyproxy/protoc-gen-validate v1.0.1
	github.com/fmeum/dep_on_gazelle v1.0.0
	github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2
	github.com/stretchr/testify v1.6.1
	golang.org/x/sys v0.15.0
)

require (
	github.com/bazelbuild/bazel-gazelle v0.30.0 // indirect
	github.com/davecgh/go-spew v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
