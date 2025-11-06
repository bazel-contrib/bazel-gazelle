// This will stop go mod from descending into this directory.
module github.com/bazelbuild/bazel-gazelle/tests/bcr/go_mod

go 1.24.2

// Validate go.mod replace directives can be properly used:
replace github.com/bmatcuk/doublestar/v4 => github.com/bmatcuk/doublestar/v4 v4.9.1

require (
	example.org/hello v1.0.0
	github.com/DataDog/sketches-go v1.4.1
	github.com/bazelbuild/buildtools v0.0.0-20251031164759-f48b23493530
	github.com/bazelbuild/rules_go v0.58.3
	// NOTE: keep <4.7.0 to test the 'replace'
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/cloudflare/circl v1.3.7
	github.com/envoyproxy/protoc-gen-validate v1.2.1
	github.com/fmeum/dep_on_gazelle v1.0.0
	github.com/google/go-jsonnet v0.20.0
	github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2
	github.com/stretchr/testify v1.8.0
	golang.org/x/sys v0.31.0
	google.golang.org/genproto v0.0.0-20250115164207-1a7da9e5054f
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20250929231259-57b25ae835d4
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/bazelbuild/bazel-gazelle v0.30.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250313205543-e70fdf4c4cb4 // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.6.1 // indirect
	rsc.io/quote v1.5.2 // indirect
	rsc.io/sampler v1.3.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace example.org/hello => ../../fixtures/hello

tool honnef.co/go/tools/cmd/staticcheck
tool mvdan.cc/gofumpt
