module github.com/bazelbuild/bazel-gazelle/tests/bcr/go_work

go 1.24.12

// Regression test for #2394: consumer imports a subpackage of "pkg" below. The indirect
// require mirrors what a real `go mod tidy` produces once something transitively needs
// "pkg": without it, "pkg" never enters the MVS graph before the replace_map skip below
// applies, so it's excluded from resolution entirely instead of getting duplicated.
require (
	example.org/consumer v0.0.0-00010101000000-000000000000
	github.com/bazelbuild/bazel-gazelle/tests/bcr/go_work/pkg v0.0.0-00010101000000-000000000000 // indirect
)

replace (
	example.org/consumer => ../../fixtures/consumer
	// Redundant with go.work's own "use ./pkg": this used to make go_deps fetch "pkg" as
	// an external module, duplicating the in-tree package.
	github.com/bazelbuild/bazel-gazelle/tests/bcr/go_work/pkg => ./pkg
)
