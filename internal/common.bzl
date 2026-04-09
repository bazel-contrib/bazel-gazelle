# Copyright 2021 The Bazel Authors. All rights reserved.
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

def env_execute(ctx, arguments, environment = {}, **kwargs):
    """Executes a command in for a repository rule.
    It prepends "env -i" to "arguments" before calling "ctx.execute".
    Variables that aren't explicitly mentioned in "environment"
    are removed from the environment. This should be preferred to "ctx.execute"
    in most situations.
    """
    if ctx.os.name.startswith("windows"):
        return ctx.execute(arguments, environment = environment, **kwargs)
    env_args = ["env", "-i"]
    environment = dict(environment)
    for var in ["TMP", "TMPDIR"]:
        if var in ctx.os.environ and not var in environment:
            environment[var] = ctx.os.environ[var]
    for k, v in environment.items():
        env_args.append("%s=%s" % (k, v))
    arguments = env_args + arguments
    return ctx.execute(arguments, **kwargs)

def executable_extension(ctx):
    extension = ""
    if ctx.os.name.startswith("windows"):
        extension = ".exe"
    return extension

def watch(ctx, path):
    # Versions of Bazel that have ctx.watch may no longer explicitly watch
    # labels on which ctx.path is called and/or labels in attributes. Do so
    # explicitly here, duplicate watches are no-ops.
    if hasattr(ctx, "watch"):
        ctx.watch(path)

def getenv(repo_ctx, name, default = ""):
    # Use repo_ctx.getenv if it exists (after Bazel 7.1.0) to also invalidate
    # the repository rule when the env var changes.
    # We can remove this wrapper after the minimal supported Bazel version is >= 7.1.0.
    if hasattr(repo_ctx, "getenv"):
        return repo_ctx.getenv(name, default)
    return repo_ctx.os.environ.get(name, default)

def resolve_env(ctx, direct = {}, inherit = [], reserved = []):
    """Builds an environment map from explicit values and host env passthroughs."""
    env = {}
    reserved_keys = {key: True for key in reserved}
    inherited_keys = {}
    for key, value in direct.items():
        if key in reserved_keys:
            fail("{} cannot be set in go_env".format(key))
        env[key] = value

    for key in inherit:
        if key in inherited_keys:
            continue
        inherited_keys[key] = True
        if key in reserved_keys:
            fail("{} cannot be set in go_env_inherit".format(key))
        if key in env:
            fail("{} cannot be set in both go_env and go_env_inherit".format(key))

        # Use getenv when available so repository rules invalidate if the
        # underlying host environment changes.
        value = getenv(ctx, key, None)
        if value != None:
            env[key] = value

    return env
