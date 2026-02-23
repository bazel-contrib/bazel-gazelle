go_select_labels_sorted checks that string lists in srcs and deps are sorted
using buildifier order, even if they are inside select expressions.
This applies to both new and existing lists and should preserve comments.
buildifier does not do this yet bazelbuild/buildtools#122, so we do this
in addition to calling build.Rewrite.
