Tests that `alias_kind` and `map_kind` work together when `alias_kind` names a
kind that `map_kind` also rewrites.

`alias_kind` refers to the wrapped kind by its original name (`go_test`,
`go_library`), but by the time rules are merged, generated rules carry the
mapped name (`go_custom_test`, `my_library`). Gazelle must compare the two in
the same namespace, otherwise the existing macro invocations are left untouched
and a duplicate rule of the mapped kind is added.

See https://github.com/bazel-contrib/bazel-gazelle/issues/2313.
