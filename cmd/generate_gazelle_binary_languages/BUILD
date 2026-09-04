load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "generate_gazelle_binary_languages_lib",
    srcs = ["main.go"],
    importpath = "github.com/bazelbuild/bazel-gazelle/cmd/generate_gazelle_binary_languages",
    visibility = ["//visibility:private"],
    deps = ["@org_golang_x_tools//go/gcexportdata"],
)

go_binary(
    name = "generate_gazelle_binary_languages",
    embed = [":generate_gazelle_binary_languages_lib"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "generate_gazelle_binary_languages_test",
    srcs = ["main_test.go"],
    embed = [":generate_gazelle_binary_languages_lib"],
)

filegroup(
    name = "all_files",
    testonly = True,
    srcs = [
        "BUILD",
        "main.go",
        "main_test.go",
    ],
    visibility = ["//visibility:public"],
)
