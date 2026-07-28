load("@io_bazel_rules_go//go:def.bzl", "go_context")
load(
    "@io_bazel_rules_go//go/private:context.bzl",
    "CGO_ATTRS",
    "CGO_FRAGMENTS",
    "CGO_TOOLCHAINS",
)

def _std_package_list_impl(ctx):
    go = go_context(ctx)
    args = ctx.actions.args()
    args.add_all([go.sdk.package_list, ctx.outputs.out])
    ctx.actions.run(
        inputs = [go.sdk.package_list],
        outputs = [ctx.outputs.out],
        executable = ctx.executable._gen_std_package_list,
        arguments = [args],
        mnemonic = "GoStdPackageList",
    )
    return [DefaultInfo(files = depset([ctx.outputs.out]))]

std_package_list = rule(
    implementation = _std_package_list_impl,
    attrs = {
        "out": attr.output(mandatory = True),
        "_gen_std_package_list": attr.label(
            default = "//language/go/gen_std_package_list",
            cfg = "exec",
            executable = True,
        ),
        "_go_context_data": attr.label(
            default = "@io_bazel_rules_go//:go_context_data",
        ),
    } | CGO_ATTRS,
    fragments = CGO_FRAGMENTS,
    toolchains = ["@io_bazel_rules_go//go:toolchain"] + CGO_TOOLCHAINS,
)
