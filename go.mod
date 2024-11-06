module github.com/bazelbuild/bazel-gazelle

go 1.22.0

toolchain go1.23.2

require (
	github.com/bazelbuild/buildtools v0.0.0-20240918101019-be1c24cc9a44
	github.com/bazelbuild/rules_go v0.50.1
	github.com/bmatcuk/doublestar/v4 v4.7.1
	github.com/fsnotify/fsnotify v1.8.0
	github.com/google/go-cmp v0.6.0
	github.com/pmezard/go-difflib v1.0.0
	golang.org/x/mod v0.21.0
	golang.org/x/sync v0.8.0
	golang.org/x/tools/go/vcs v0.1.0-deprecated
)

require golang.org/x/sys v0.26.0 // indirect
