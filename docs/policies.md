# Policies for drop installs

drop verifies every artifact it installs against AMPEL policies before the
file touches the system. This document describes where those policies come
from and the data drop hands them, so a policy can react to the release being
installed.

## Where policies come from

For an artifact from `github.com/<org>/<repo>`, drop reads policy sets from
the organization's `.github` repository:

| Directory | Applies to |
| --- | --- |
| `ampel/policies/release/<repo>/` | releases of that repository |
| `ampel/policies/release/_/` | releases of every repository in the organization |

Both directories are read and every policy set found in them is evaluated.
When neither yields a policy set, drop falls back to the community policies in
[policylabs/oss](https://github.com/policylabs/oss) under
`policies/<org>/<repo>/release/`. An explicit `--policy-repo` replaces the
organization's repository and disables the fallback.

Policy sets can be written in JSON or HJSON, as bare files or wrapped in
attestations (the output of `policyctl sign`). References to other policies,
such as the library in [carabiner-dev/policies](https://github.com/carabiner-dev/policies),
are resolved at install time. A reference that cannot be fetched fails the
install rather than silently verifying against fewer policies.

## Context values drop injects

Policies can declare context values and read them in their tenets. drop
supplies these values about the artifact being verified:

| Context value | Contents | Example |
| --- | --- | --- |
| `drop_host` | GitHub host | `github.com` |
| `drop_org` | repository owner | `carabiner-dev` |
| `drop_repo` | repository name | `drop` |
| `drop_repository` | repository URL | `https://github.com/carabiner-dev/drop` |
| `drop_tag` | release tag as published | `v1.2.3-rc.1` |
| `drop_version` | release tag without the leading `v` | `1.2.3-rc.1` |
| `drop_asset` | name of the release asset being verified | `drop-v1.2.3-linux-amd64` |
| `drop_os` | operating system of the asset, when drop knows it | `linux` |
| `drop_arch` | architecture of the asset, when drop knows it | `amd64` |

A policy only sees the values it declares. Declare them in the policy set's
`common.context` to expose them to every policy in the set, or in one policy's
`context` block:

```hjson
common: {
    context: {
        drop_version: {
            type: "string"
            required: true
            description: "Version of the release being verified (injected by drop)"
        }
    }
}
```

Values burned into the policy with `value:` cannot be overridden, so leave
the injected ones without a value. Marking them `required` makes the policy
fail loudly when evaluated by a tool that does not supply them.

The tag and version come from the GitHub release drop is installing from, not
from the attestations. They identify what the user asked for; the evidence
about what was built still comes from the attestations the policy admits.

## Branching on the version

A policy declares the releases it applies to with a `when` condition, an
expression evaluated against the context values before any evidence is
fetched. When it is false the policy is **skipped**: drop prints it as
`⏭️ SKIP` with the condition, and it does not count against the artifact.
AMPEL's CEL runtime ships a `semver` plugin, so conditions can compare the
version being installed against ranges and prerelease labels. This is how a
policy copes with a project whose security metadata changed between
releases: old releases are held to what they shipped, new ones to the
current bar, and the policy set stays a single file.

Require a newer attestation format from a version on, while still verifying
older releases with what they carry:

```hjson
{
    id: current-provenance
    when: { expression: "semver.satisfies(context.drop_version, '>=2.0.0')" }
    tenets: [
        {
            id: has-slsa-v1
            predicates: { types: ["https://slsa.dev/provenance/v1"] }
            code: "size(predicates) > 0"
        }
    ]
}
{
    id: legacy-provenance
    when: { expression: "semver.satisfies(context.drop_version, '<2.0.0')" }
    tenets: [
        {
            id: has-slsa
            predicates: { types: ["https://slsa.dev/provenance/v0.2", "https://slsa.dev/provenance/v1"] }
            code: "size(predicates) > 0"
        }
    ]
}
```

A condition can also gate a policy referenced from the shared library
without editing it, since `when` on the referencing stanza is applied to the
referenced policy:

```hjson
{
    id: slsa-builder-id
    source: { location: { uri: "git+https://github.com/carabiner-dev/policies@<commit>#slsa/slsa-builder-id.json" } }
    when: { expression: "semver.satisfies(context.drop_version, '>=2.0.0')" }
}
```

Treat prereleases differently from stable releases:

```hjson
when: { expression: "semver.prerelease(context.drop_version) == ''" }
```

Conditions see the context values in scope, the subject and the runtime
plugins, but no attestations: applicability is a property of what is being
installed, not of the evidence found for it. Every context value a condition
reads must be declared, and the evaluation fails if it is not.

When **every** policy skips, nothing was verified. drop does not treat that
as a pass: the install stops with "No verification policy applies to
<org>/<repo> <tag>", and the same `--policy-repo` and `--insecure` ways
forward as when a project has no policies at all.

Pin evidence to the exact release by reusing the tag in an identity or a
context value with `fromContext`, for example when the release workflow signs
with an identity that carries the tag:

```hjson
identities: [
    {
        sigstore: {
            issuerMatch: { exact: "https://token.actions.githubusercontent.com" }
            identityMatch: { fromContext: "release_identity" }
        }
    }
]
context: {
    drop_tag: { type: "string", required: true }
    release_identity: {
        expression: "'https://github.com/carabiner-dev/drop/.github/workflows/release.yaml@refs/tags/' + context.drop_tag"
    }
}
```

The full list of helpers (`semver.satisfies`, `semver.compare`,
`semver.isStable`, `semver.major`, ...) is in the AMPEL
[CEL plugins reference](https://github.com/carabiner-dev/ampel/blob/main/docs/cel-plugins.md),
and conditions are described in the
[policy guide](https://github.com/carabiner-dev/ampel/blob/main/docs/03-ampel-policy-guide.md).

## Testing a policy outside drop

The ampel CLI evaluates the same policy against a release; supply the values
drop would inject with `-x`:

```
ampel verify ./drop-v0.0.1-linux-amd64 \
    -c release:github.com/carabiner-dev/drop@v0.0.1 \
    -p ampel/policies/release/drop/drop-release.hjson \
    -x drop_version:0.0.1 -x drop_tag:v0.0.1
```

To test the policy through drop itself before publishing it, point drop at a
local checkout of the policy repository (its committed contents are read):

```
drop get --policy-repo ./dot-github carabiner-dev/drop@v0.0.1
```

drop's own release policy, which gates two provenance policies on the
version so early prereleases are verified with their SLSA v0.2 provenance
while stable releases must ship v1, lives in
[carabiner-dev/.github](https://github.com/carabiner-dev/.github/blob/main/ampel/policies/release/drop/drop-release.hjson).
