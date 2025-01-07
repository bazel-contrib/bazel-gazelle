// This will stop go mod from descending into this directory.
module github.com/bazelbuild/bazel-gazelle/tests/bcr/go_mod

go 1.23.3

// Validate go.mod replace directives can be properly used:
replace github.com/bmatcuk/doublestar/v4 => github.com/bmatcuk/doublestar/v4 v4.7.1

require (
	example.org/hello v1.0.0
	github.com/DataDog/sketches-go v1.4.1
	// NOTE: Keep at v1.25.0 to test rewriting of load statements
	github.com/bazelbuild/bazelisk v1.25.0
	github.com/bazelbuild/buildtools v0.0.0-20230317132445-9c3c1fc0106e
	github.com/bazelbuild/rules_go v0.39.1
	// NOTE: keep <4.7.0 to test the 'replace'
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/cloudflare/circl v1.3.7
	github.com/envoyproxy/protoc-gen-validate v1.0.1
	github.com/fmeum/dep_on_gazelle v1.0.0
	github.com/google/go-jsonnet v0.20.0
	github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2
	github.com/stretchr/testify v1.9.0
	golang.org/x/sys v0.27.0
)

require google.golang.org/protobuf v1.32.0

require (
	github.com/bazelbuild/bazel-gazelle v0.30.0 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/term v0.26.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	rsc.io/quote v1.5.2 // indirect
	rsc.io/sampler v1.3.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace example.org/hello => ../../fixtures/hello
