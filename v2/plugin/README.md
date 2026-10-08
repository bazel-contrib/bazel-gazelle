# Language plugins in any language

Gazelle can use language extensions that run as separate processes. A plugin
is an executable written in any language: Gazelle starts it, then exchanges
JSON-RPC messages with it over the plugin's stdin and stdout. This lets a
TypeScript plugin use the TypeScript compiler API, a Python plugin use Python's
`ast` module, and so on, without rebuilding Gazelle and without writing Go.

Plugins are used exactly like extensions compiled into Gazelle with
`gazelle_binary`: their rules are merged into build files, their directives are
recognized, their libraries are indexed, and their dependencies are resolved
against the same index as every other language (in both directions).
`-lang`, `# gazelle:lang`, `# gazelle:map_kind`, `# gazelle:resolve`, and
`# keep` all work with plugins.

* [Using a plugin](#using-a-plugin)
* [Writing a plugin](#writing-a-plugin)
* [Protocol reference](#protocol-reference)
* Reference plugins: [examples/sh/sh_plugin.py](examples/sh/sh_plugin.py)
  (Python, standard library only) and [examples/sh/sh.go](examples/sh/sh.go)
  (Go, using [server](server)).

## Using a plugin

Pass the plugin executable with `-plugin`. The flag may be repeated.

```sh
gazelle -plugin=tools/gazelle/sh_plugin.py
```

With Bazel, list the plugin in the `data` attribute of the `gazelle` rule and
pass its runfiles path:

```starlark
load("@gazelle//:def.bzl", "gazelle")

gazelle(
    name = "gazelle",
    data = ["//tools/gazelle:sh_plugin"],
    extra_args = ["-plugin=$(rlocationpath //tools/gazelle:sh_plugin)"],
)
```

A relative `-plugin` path is looked up in Bazel runfiles, then relative to the
current directory, then relative to the workspace directory, then on `PATH`
(for names without a slash). Gazelle passes its runfiles environment variables
to the plugin, so plugins built by Bazel (for example, `py_binary` or
`js_binary` targets) can find their own runfiles.

Plugins may also be registered in the repository root build file. Paths are
relative to the repository root, and any further words are passed to the plugin
as arguments:

```starlark
# gazelle:plugin tools/gazelle/sh_plugin.py
```

## Writing a plugin

A plugin is a program that reads JSON-RPC 2.0 requests from stdin and writes
responses to stdout, one JSON object per line. It may log to stderr; Gazelle
passes stderr through to the user. It must exit when stdin is closed.

A run looks like this:

1. Gazelle parses flags, then starts each plugin and sends `initialize`. The
   plugin replies with its language name, the kinds of rules it generates, the
   directives it understands, and optional capabilities.
2. Gazelle walks the repository. For each directory (parents first), it sends
   `configure` with the directory's build file and the plugin's configuration
   value for the parent directory. The plugin returns its configuration value
   for this directory: any JSON value, which Gazelle stores and sends back with
   later requests for the directory. Plugins don't need to keep per-directory
   state.
3. In each directory Gazelle updates, it sends `fix` (optional) and `generate`.
   `generate` returns new rules, empty rules (rules that may be deleted), and
   an opaque `imports` value per rule.
4. As it indexes rules for dependency resolution, Gazelle sends `imports` for
   each rule of one of the plugin's kinds. The plugin returns the strings by
   which the rule may be imported.
5. After the walk, Gazelle sends `onResolve` (optional), then `resolve` for each
   rule the plugin generated, with the rule's `imports` value. To find
   dependencies, the plugin sends `index/find` requests back to Gazelle while
   handling `resolve`. The result describes changes to the rule, usually
   setting `deps`.
6. Gazelle writes build files, sends `onFinish` (optional), then `shutdown`, and
   closes stdin.

Requests are sent one at a time. Gazelle waits for a response before sending
the next request, and the only request a plugin may send is `index/find`, while
it's handling `resolve` or `find`.

Here is a complete exchange for a directory with one script (Gazelle's messages
are marked `>`, the plugin's `<`; some fields are omitted):

```
> {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"repoRoot":"/src/repo",...}}
< {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"name":"sh","kinds":[{"name":"sh_binary","loadedFrom":"@rules_shell//shell:sh_binary.bzl","mergeableAttrs":["srcs"],"resolveAttrs":["deps"]}],"knownDirectives":["sh_enabled"],"capabilities":{"configure":"directives"}}}
> {"jsonrpc":"2.0","id":2,"method":"generate","params":{"config":null,"dir":"/src/repo/bin","rel":"bin","file":null,"subdirs":[],"regularFiles":["run.sh"],"genFiles":[]}}
< {"jsonrpc":"2.0","id":2,"result":{"gen":[{"kind":"sh_binary","name":"run","attrs":{"srcs":["run.sh"]}}],"imports":[["lib/util.sh"]]}}
> {"jsonrpc":"2.0","id":3,"method":"resolve","params":{"rel":"bin","rule":{"kind":"sh_binary","name":"run","attrs":{"srcs":["run.sh"]}},"from":"//bin:run","imports":["lib/util.sh"]}}
< {"jsonrpc":"2.0","id":"py-1","method":"index/find","params":{"import":{"lang":"sh","imp":"lib/util.sh"}}}
> {"jsonrpc":"2.0","id":"py-1","result":{"results":[{"label":"//lib:util","relLabel":"//lib:util"}]}}
< {"jsonrpc":"2.0","id":3,"result":{"attrs":{"deps":["//lib:util"]}}}
> {"jsonrpc":"2.0","id":4,"method":"shutdown","params":{}}
< {"jsonrpc":"2.0","id":4,"result":{}}
```

Plugins written in Go can use package [server](server), which handles the
connection and dispatches requests to methods. The Go types in package
[protocol](protocol) are the schema for every message.

### Rules and attribute values

Rules are sent as `{"kind": ..., "name": ..., "attrs": {...}, "attrExprs": {...}}`.
Attribute values that can be written as plain JSON (strings, booleans,
integers, lists, and dicts with string keys) go in `attrs`. Anything else, like
a `select()` or `glob()` call, goes in `attrExprs` as Starlark source:

```json
{
  "kind": "sh_library",
  "name": "util",
  "attrs": {"srcs": ["util.sh"], "visibility": ["//visibility:public"]},
  "attrExprs": {"data": "glob([\"*.txt\"])"}
}
```

Rules from existing build files are sent the same way, with `"keep": true` for
rules marked `# keep`. Results that modify rules (`resolve`, `fix`) use `attrs`,
`attrExprs`, and `deleteAttrs`.

### Errors

A plugin reports a failed request with a JSON-RPC error response. Problems
that don't prevent a result (a syntax error in one source file, an
unresolved import) go in the `errors` list of the result:

```json
{"errors": [{"message": "bin/run.sh: no rule provides lib/missing.sh", "severity": "warning"}]}
```

`severity` is `"error"` (the default), `"warning"`, or `"critical"`, with the same
meaning as for built-in extensions: Gazelle exits non-zero after errors in
`-strict` mode, never fails for warnings, and stops after critical errors. The
same severities may be set in a JSON-RPC error's `data.severity`.

If a plugin crashes or writes something that isn't a JSON-RPC message, Gazelle
reports a critical error, prints the plugin's exit status, and exits without
writing build files.

### Versioning

`initialize` carries the protocol version. Gazelle sends the highest version it
supports; the plugin replies with the version it implements, which must not be
higher. This is version 1. New optional fields may be added to messages without
changing the version, so plugins and Gazelle must ignore fields they don't
recognize. Optional features are negotiated with capabilities.

## Protocol reference

The Go types in [protocol/protocol.go](protocol/protocol.go) are the
authoritative schema; each field is documented there. Field names are
lowerCamelCase.

| Method | Direction | Sent | Params → Result |
| --- | --- | --- | --- |
| `initialize` | Gazelle → plugin | once, first | `InitializeParams` → `InitializeResult` |
| `configure` | Gazelle → plugin | per directory, parents first (see `capabilities.configure`) | `ConfigureParams` → `ConfigureResult` |
| `fix` | Gazelle → plugin | per updated directory with a build file, if `capabilities.fix` | `FixParams` → `FixResult` |
| `generate` | Gazelle → plugin | per updated directory | `GenerateParams` → `GenerateResult` |
| `imports` | Gazelle → plugin | per indexed rule of the plugin's kinds | `ImportsParams` → `ImportsResult` |
| `onResolve` | Gazelle → plugin | after the walk, if `capabilities.lifecycle` | `{}` → `{}` |
| `resolve` | Gazelle → plugin | per rule the plugin generated | `ResolveParams` → `ResolveResult` |
| `find` | Gazelle → plugin | when an import isn't in the index, if `capabilities.find` | `FindParams` → `FindResult` |
| `onFinish` | Gazelle → plugin | after build files are written, if `capabilities.lifecycle` | `{}` → `{}` |
| `shutdown` | Gazelle → plugin | once, last | `{}` → `{}` |
| `index/find` | plugin → Gazelle | while handling `resolve` or `find` | `IndexFindParams` → `IndexFindResult` |

Capabilities returned by `initialize`:

* `configure`: which directories `configure` is sent for. `"all"` (default),
  `"buildFiles"` (directories with a build file), `"directives"` (build files
  containing one of the plugin's known directives), or `"none"`.
* `fix`: the plugin handles `fix`.
* `find`: the plugin handles `find`, resolving imports for any language that
  aren't in the index (like `CrossResolver` in the Go API).
* `lifecycle`: the plugin handles `onResolve` and `onFinish`.

`index/find` applies `# gazelle:resolve` and `# gazelle:resolve_regexp`
directives first, then searches the rule index, then asks extensions that
implement `find`. During `resolve`, each match includes `relLabel`, the label
written relative to the rule being resolved, and `selfImport`, which is true if
the match is the rule itself or a rule it embeds.

The protocol mirrors the Go extension API in
[v2/language](../language), [v2/config](../config), and
[v2/resolve](../resolve):

| Go API | Protocol |
| --- | --- |
| `Language.Name` | `InitializeResult.name` |
| `Generator.Kinds` | `InitializeResult.kinds` |
| `Configurer.KnownDirectives` | `InitializeResult.knownDirectives` |
| `Configurer.Configure` | `configure` (configuration is a JSON value instead of `Config.Exts`) |
| `Fixer.Fix` | `fix` |
| `Generator.Generate` | `generate` |
| `Indexer.Imports` | `imports` |
| `Resolver.Resolve` | `resolve`, with `RuleIndex.Find` available as `index/find` |
| `Finder.Find` | `find` |
| `OnResolver`, `OnFinisher` | `onResolve`, `onFinish` |
