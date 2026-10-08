#!/usr/bin/env python3
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

"""Reference Gazelle plugin for shell scripts, written in Python.

This is the same plugin as sh.go, written without any dependencies beyond
the Python standard library, to show how a plugin implements Gazelle's
subprocess plugin protocol directly. Run Gazelle with
-plugin=path/to/sh_plugin.py to use it.

The protocol is JSON-RPC 2.0. Each message is one line of JSON on stdin
(requests from Gazelle) or stdout (responses, and index/find requests sent
back to Gazelle). Logs go to stderr. See v2/plugin/README.md.
"""

import json
import os
import posixpath
import sys

PROTOCOL_VERSION = 1
LANG = "sh"
ENABLED_DIRECTIVE = "sh_enabled"


def kind(name, bzl):
    return {
        "name": name,
        "loadedFrom": "@rules_shell//shell:" + bzl,
        "nonEmptyAttrs": ["srcs", "deps"],
        "mergeableAttrs": ["srcs"],
        "resolveAttrs": ["deps"],
    }


KINDS = [
    kind("sh_binary", "sh_binary.bzl"),
    kind("sh_library", "sh_library.bzl"),
    kind("sh_test", "sh_test.bzl"),
]
KIND_NAMES = {k["name"] for k in KINDS}


class RpcError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Conn:
    """A JSON-RPC 2.0 connection framed as JSON Lines."""

    def __init__(self, infile, outfile):
        self.infile = infile
        self.outfile = outfile
        self.next_id = 0

    def read(self):
        while True:
            line = self.infile.readline()
            if not line:
                return None
            if line.strip():
                return json.loads(line)

    def write(self, msg):
        msg["jsonrpc"] = "2.0"
        self.outfile.write(json.dumps(msg, separators=(",", ":")) + "\n")
        self.outfile.flush()

    def call(self, method, params):
        """Sends a request to Gazelle and waits for the response."""
        self.next_id += 1
        req_id = "py-%d" % self.next_id
        self.write({"id": req_id, "method": method, "params": params})
        msg = self.read()
        if msg is None:
            raise EOFError("Gazelle closed the connection")
        if msg.get("id") != req_id or "method" in msg:
            raise RuntimeError("unexpected message while waiting for %s: %r" % (method, msg))
        if "error" in msg:
            raise RpcError(msg["error"].get("code", -32603), msg["error"].get("message", ""))
        return msg.get("result") or {}


def parse_config(raw):
    cfg = {"enabled": True}
    if isinstance(raw, dict):
        cfg.update(raw)
    return cfg


def initialize(conn, params):
    return {
        "protocolVersion": PROTOCOL_VERSION,
        "name": LANG,
        "kinds": KINDS,
        "knownDirectives": [ENABLED_DIRECTIVE],
        "capabilities": {"configure": "directives"},
    }


def configure(conn, params):
    cfg = parse_config(params.get("config"))
    errors = []
    f = params.get("file")
    for d in (f or {}).get("directives") or []:
        if d["key"] != ENABLED_DIRECTIVE:
            continue
        value = d["value"].strip()
        if value in ("true", "false"):
            cfg["enabled"] = value == "true"
        else:
            errors.append({
                "message": "%s: gazelle:%s: expected true or false, got %s"
                % (f["path"], ENABLED_DIRECTIVE, json.dumps(d["value"])),
            })
    return {"config": cfg, "errors": errors}


def parse_sources(content, rel):
    paths = set()
    for line in content.split("\n"):
        fields = line.split()
        if len(fields) < 2 or fields[0] not in ("source", "."):
            continue
        p = fields[1].strip("\"'")
        if not p or "$" in p or p.startswith("/"):
            continue
        if p.startswith("./") or p.startswith("../"):
            p = posixpath.normpath(posixpath.join(rel, p))
        paths.add(p)
    return sorted(paths)


def generate(conn, params):
    cfg = parse_config(params.get("config"))
    if not cfg["enabled"]:
        return {}
    rel = params["rel"]
    gen, imports, errors = [], [], []
    generated = set()
    for script in sorted(f for f in params.get("regularFiles") or [] if f.endswith(".sh")):
        try:
            with open(os.path.join(params["dir"], script), encoding="utf-8", errors="replace") as fh:
                content = fh.read()
        except OSError as e:
            errors.append({"message": str(e)})
            continue
        name = posixpath.basename(script)[: -len(".sh")]
        if name.endswith("_test"):
            k = "sh_test"
        elif content.startswith("#!"):
            k = "sh_binary"
        else:
            k = "sh_library"
        attrs = {"srcs": [script]}
        if k == "sh_library":
            attrs["visibility"] = ["//visibility:public"]
        gen.append({"kind": k, "name": name, "attrs": attrs})
        imports.append(parse_sources(content, rel))
        generated.add(name)

    # Rules for scripts that no longer exist are reported as empty, so Gazelle
    # can delete them.
    empty = []
    for r in (params.get("file") or {}).get("rules") or []:
        if r["kind"] in KIND_NAMES and r.get("name") and r["name"] not in generated:
            empty.append({"kind": r["kind"], "name": r["name"]})
    return {"gen": gen, "empty": empty, "imports": imports, "errors": errors}


def imports(conn, params):
    r = params["rule"]
    if r["kind"] == "sh_library":
        srcs = (r.get("attrs") or {}).get("srcs") or []
        rel = params["rel"]
        return {
            "imports": [
                {"lang": LANG, "imp": posixpath.normpath(posixpath.join(rel, s))}
                for s in srcs
                if isinstance(s, str)
            ]
        }
    if r["kind"] in ("sh_binary", "sh_test"):
        return {"notImportable": True}
    return {}


def resolve(conn, params):
    deps, errors = set(), []
    for imp in params.get("imports") or []:
        found = conn.call("index/find", {"import": {"lang": LANG, "imp": imp}})
        results = found.get("results") or []
        matched = [m["relLabel"] for m in results if not m.get("selfImport")]
        if not matched and not any(m.get("selfImport") for m in results):
            errors.append({
                "message": "%s: no rule provides sourced file %s" % (params["from"], imp),
                "severity": "warning",
            })
        deps.update(matched)
    result = {"errors": errors}
    if deps:
        result["attrs"] = {"deps": sorted(deps)}
    return result


def empty(conn, params):
    return {}


HANDLERS = {
    "initialize": initialize,
    "configure": configure,
    "generate": generate,
    "imports": imports,
    "resolve": resolve,
    "shutdown": empty,
}


def main():
    conn = Conn(sys.stdin, sys.stdout)
    while True:
        msg = conn.read()
        if msg is None:
            return 0
        if "method" not in msg:
            print("sh_plugin: unexpected response: %r" % msg, file=sys.stderr)
            return 1
        handler = HANDLERS.get(msg["method"])
        try:
            if handler is None:
                raise RpcError(-32601, "method not found: " + msg["method"])
            response = {"result": handler(conn, msg.get("params") or {})}
        except RpcError as e:
            response = {"error": {"code": e.code, "message": e.message}}
        except Exception as e:  # Report bugs to Gazelle instead of crashing.
            response = {"error": {"code": -32603, "message": "%s: %s" % (type(e).__name__, e)}}
        if "id" in msg:
            response["id"] = msg["id"]
            conn.write(response)


if __name__ == "__main__":
    sys.exit(main())
