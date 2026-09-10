# Copyright 2026 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"Helper functions for go_deps related to the GOPROXY protocol"

load(":globs.bzl", "match_prefix_patterns")

def required_mod_file(
        importpath,
        version):
    """
    Tracks information about a go.mod file that 'go list -m' will need.

    Args:
        importpath: the Go module path
        version: the required version. We generally only need the highest.

    Return:
        A struct with the same information, plus a sha256 field with the sum
        in Bazel's hex format.
    """
    return struct(
        importpath = importpath,
        version = version,
    )

def download_mod_files(
        module_ctx,
        go_env,
        required_mod_files):
    """
    Downloads a set of .mod files using module_ctx.download

    We prefer to download .mod files with Bazel's downloader so that they're
    covered by --downloader_config and --experimental_remote_download  and can
    be reused from the repository cache.

    This is best effort: we can't predict ahead of time all the .mod files that
    'go list -m' will need. We only attempt to download from the first proxy
    in the list (only if http: or https:). We only attempt to download modules
    that are NOT private according to GOPRIVATE and GONOPROXY.

    Args:
        module_ctx: the module context.
        go_env: the Go environment. GOPROXY and GONOPROXY must be set.
        required_mod_files: dict mapping required_mod_file struct to sha256 sum.
            If set, download_mod_files passes this to module_ctx.download, so
            the download fails if the file changed upstream.
            NOTE: this is different from the hash in go.sum.

    Returns:
        A tuple:
        - download_dir: Path to a temporary directory that can be used as a
          file:// proxy containing downloads.
        - new_sha256: same format as required_mod_files. Contains an entry with
          a sha256 sum for each entry in required_mod_files that was missing a sum.
    """
    download_dir = module_ctx.path("modcache/cache/download")

    # Check if the first GOPROXY is an HTTP server. We won't attempt to download
    # from any other type of proxy.
    comma = go_env["GOPROXY"].find(",")
    pipe = go_env["GOPROXY"].find("|")
    if comma >= 0 and (pipe < 0 or comma < pipe):
        goproxy = go_env["GOPROXY"][:comma]
    elif pipe >= 0:
        goproxy = go_env["GOPROXY"][:pipe]
    else:
        goproxy = go_env["GOPROXY"]
    goproxy = goproxy.rstrip("/")
    if ((not goproxy.startswith("https://") and not goproxy.startswith("http://")) or
        "@" in goproxy):
        return download_dir, {}

    # Download all the .mod files.
    # Synthesize .info files. The content doesn't affect version selection, but
    # 'go list -m' wants them anyway.
    # Synthesize a list file for all the versions downloaded.
    downloads = []
    mod_versions = {}  # importpath => list of versions
    for m, sha256 in required_mod_files.items():
        if match_prefix_patterns(go_env["GONOPROXY"], m.importpath):
            # Don't disclose private module path to proxy.
            continue
        escaped_importpath = _escape_mod_case(m.importpath)
        escaped_version = _escape_mod_case(m.version)
        url = "{goproxy}/{importpath}/@v/{version}.mod".format(
            goproxy = goproxy,
            importpath = escaped_importpath,
            version = escaped_version,
        )
        mod_output = download_dir.get_child(escaped_importpath, "@v", escaped_version + ".mod")
        dl = module_ctx.download(
            url = url,
            output = mod_output,
            sha256 = sha256 if sha256 else "",
            allow_fail = True,
            block = False,
        )
        downloads.append((m, dl))
        info_output = download_dir.get_child(escaped_importpath, "@v", escaped_version + ".info")
        module_ctx.file(info_output, json.encode({"Version": m.version}))
        if m.importpath not in mod_versions:
            mod_versions[m.importpath] = []
        if not is_pseudoversion(m.version):
            mod_versions[m.importpath].append(m.version)

    new_sha256 = {}
    for m, dl in downloads:
        result = dl.wait()
        if result.success and not required_mod_files[m]:
            new_sha256[m] = result.sha256

    # Synthesize list files
    for importpath, versions in mod_versions.items():
        list_output = module_ctx.path("modcache/cache/download/{}/@v/list".format(importpath))
        module_ctx.file(list_output, "\n".join(versions) + "\n")

    return module_ctx.path("modcache/cache/download"), new_sha256

_MOD_FILE_FACTS_KEY = "required_mod_files_v1"

def make_mod_file_facts(required_mod_files):
    """
    Return a MODULE.bazel.lock facts dict with hashes of required .mod files

    Storing these facts helps us fetch all necessary files with
    module_ctx.download before running 'go list -m'. We can't predict exactly
    what 'go list -m' will need from 'require' directives 'module' tags,
    so we remember instead.
    """
    mod_file_facts = [
        {
            "importpath": m.importpath,
            "version": m.version,
            "sha256": sha256,
        }
        for m, sha256 in required_mod_files.items()
        if sha256
    ]
    return {_MOD_FILE_FACTS_KEY: mod_file_facts}

def read_mod_file_facts(module_ctx):
    "Read back facts written with write_mod_file_facts"
    facts = getattr(module_ctx, "facts", None)
    if not facts:
        return {}
    mod_file_facts = facts.get(_MOD_FILE_FACTS_KEY)
    if not mod_file_facts:
        return {}
    return {
        required_mod_file(importpath = e["importpath"], version = e["version"]): e["sha256"]
        for e in mod_file_facts
    }

def _escape_mod_case(s):
    """
    Escapes a module path or version for use in a module proxy path

    Each uppercase letter is replaced with an exclamation mark followed by the
    lowercase letter, so that module paths differing only in case don't collide
    on case-insensitive file systems. See module.EscapePath in
    golang.org/x/mod.
    """
    if s == s.lower():
        return s
    return "".join([
        "!" + c.lower() if c.isupper() else c
        for c in s.elems()
    ])

_HEX = "0123456789abcdef"

def is_pseudoversion(v):
    """
    Tests whether a version string is a pseudo-version

    See golang.org/x/mod/module

    Args:
        v: version string to test

    Returns:
        Whether v is a pseudo-version.
    """
    parts = v.split("-")
    if len(parts) < 3:
        return False
    time = parts[-2]
    dot = time.rfind(".")
    if dot >= 0:
        time = time[dot + 1:]
    rev = parts[-1].rstrip("+incompatible")
    return len(time) == 14 and time.isdigit() and all([c in _HEX for c in rev.elems()])
