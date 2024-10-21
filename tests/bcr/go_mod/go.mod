// This will stop go mod from descending into this directory.
module github.com/bazelbuild/bazel-gazelle/tests/bcr/go_mod

go 1.21.5

// Validate go.mod replace directives can be properly used:
replace github.com/bmatcuk/doublestar/v4 => github.com/bmatcuk/doublestar/v4 v4.7.1

require (
	example.org/hello v1.0.0
	github.com/DataDog/sketches-go v1.4.1
	github.com/bazelbuild/buildtools v0.0.0-20230317132445-9c3c1fc0106e
	github.com/bazelbuild/rules_go v0.39.1
	// NOTE: keep <4.7.0 to test the 'replace'
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/cloudflare/circl v1.3.7
	github.com/envoyproxy/protoc-gen-validate v1.0.1
	github.com/fmeum/dep_on_gazelle v1.0.0
	github.com/google/go-jsonnet v0.20.0
	github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2
	github.com/stretchr/testify v1.6.1
	golang.org/x/sys v0.15.0
)

require (
	github.com/bazelbuild/bazel-gazelle v0.30.0 // indirect
	github.com/davecgh/go-spew v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	rsc.io/quote v1.5.2 // indirect
	rsc.io/sampler v1.3.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace example.org/hello => ../../fixtures/hello
