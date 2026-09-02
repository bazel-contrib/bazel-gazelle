#!/usr/bin/env bash
set -euo pipefail

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail; set +e; f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null || \
  source "$0.runfiles/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  { echo>&2 "ERROR: cannot find $f"; exit 1; }; f=; set -e
# --- end runfiles.bash initialization v3 ---

convert_rlocation=bazel_gazelle/tools/convert-go-deps-test-case/convert-go-deps-test-case_/convert-go-deps-test-case
convert=$(rlocation "${convert_rlocation}")
if [[ -z "${convert}" ]]; then
  echo "error: could not locate convert-go-deps-test-case binary" >&2
  exit 1
fi

# Repack's runfiles tree lacks @go_sdk//:bin/go. Clear inherited runfiles env
# vars so the convert binary discovers its own runfiles manifest instead.
unset RUNFILES_DIR RUNFILES_MANIFEST_FILE JAVA_RUNFILES

cd "${BUILD_WORKSPACE_DIRECTORY}"

tmpbase=$(mktemp -d)
trap 'rm -rf "${tmpbase}"' EXIT

for bzl in tests/bzlmod/go_deps/*.bzl; do
  name=$(basename "${bzl}" .bzl)
  tmp="${tmpbase}/${name}"
  echo "=== Repacking ${name} ==="
  "${convert}" -from="${bzl}" -to="${tmp}"
  "${convert}" -from="${tmp}" -to="${bzl}"
done

echo "All test cases repacked."
