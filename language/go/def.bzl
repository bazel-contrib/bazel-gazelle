load("@io_bazel_rules_go//go:def.bzl", "go_context")

def _std_package_list_impl(ctx):
    go = go_context(ctx)
    package_list = go.sdk.package_list
    outputs = [ctx.outputs.out]

    args = ctx.actions.args()
    args.add_all([package_list, ctx.outputs.out])

    ctx.actions.run(
        inputs = [package_list],
        outputs = outputs,
        executable = ctx.executable._gen_std_package_list,
        arguments = [args],
        mnemonic = "GoStdPackageList",
    )
    return [DefaultInfo(files = depset(outputs))]

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
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)
