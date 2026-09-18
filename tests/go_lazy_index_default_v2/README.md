Verify that in v2, lazy indexing is enabled by default.

Gazelle is invoked on a.

It should resolve a dependency on lazy/b with lazy indexing.

It should NOT resolve a dependency on not_indexed/c.
