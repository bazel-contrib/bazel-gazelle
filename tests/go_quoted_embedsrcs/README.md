Checks that go:embed directives with spaces and quotes are parsed correctly.
This probably belongs in //language/go:go_test, but we need file names with
spaces, and Bazel doesn't allow those in runfiles, which that test depends
on.