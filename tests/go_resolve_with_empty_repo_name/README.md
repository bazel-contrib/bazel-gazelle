# Resolving expressions whose `RepoName` is empty

This tests two cases of resolving import paths to ensure that the configured `RepoName` is substituted
in cases where the resolved label would have an empty `Repo` field.

Not doing this can cause a package-relative label to be turned into an absolute label.

See [#2269](https://github.com/bazel-contrib/bazel-gazelle/issues/2269) for details.
