# go_search directive

This is a test for lazy indexing in general, exercised by the Go extension.

With the `-r=false -index=lazy` flags, Gazelle only visits directories named on the command line but still indexes libraries. This should be very fast.

In order to index more libraries, the user can add search directories with:

    # gazelle:go_search <rel> <prefix>

In this test, we check two forms of this directive: a vendor directory with a non-standard name having no prefix, and a module replacement directory with a specific prefix.
