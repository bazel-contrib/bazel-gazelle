#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

# Set by GH actions, see
# https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables
TAG=${GITHUB_REF_NAME}
# The prefix is chosen to match what GitHub generates for source archives
PREFIX="bazel-gazelle-${TAG:1}"
ARCHIVE="bazel-gazelle-$TAG.tar.zst"
git archive --format=tar "${TAG}" | zstd >"$ARCHIVE"
SHA=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')

cat << EOF
## Using Bzlmod

\`\`\`starlark
bazel_dep(name = "gazelle", version = "${TAG:1}")
\`\`\`

## Using WORKSPACE

\`\`\`starlark
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
    name = "bazel-gazelle",
    sha256 = "${SHA}",
    url = "https://github.com/bazel-contrib/bazel-gazelle/releases/download/${TAG}/${ARCHIVE}",
)
load("@bazel-gazelle//:deps.bzl", "gazelle_dependencies")
gazelle_dependencies()
\`\`\`
EOF
