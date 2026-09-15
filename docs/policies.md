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

AMPEL's CEL runtime ships a `semver` plugin, so tenets can compare the version
being installed against ranges and prerelease labels. This is how a policy
copes with a project whose security metadata changed between releases: old
releases are held to what they shipped, new ones to the current bar, and the
policy stays a single file.

Require a newer attestation format from a version on, while still verifying
older releases with what they carry:

```hjson
{
    id: provenance-generation
    meta: {
        description: "Releases from 2.0.0 ship SLSA v1 provenance, older ones may ship v0.2"
        assert_mode: "OR"
    }
    tenets: [
        {
            id: current-releases
            predicates: { types: ["https://slsa.dev/provenance/v1"] }
            code: "semver.satisfies(context.drop_version, '>=2.0.0') && size(predicates) > 0"
        }
        {
            id: legacy-releases
            predicates: { types: ["https://slsa.dev/provenance/v0.2", "https://slsa.dev/provenance/v1"] }
            code: "semver.satisfies(context.drop_version, '<2.0.0') && size(predicates) > 0"
        }
    ]
}
```

With `assert_mode: OR` one passing tenet is enough. A tenet whose predicate
types match nothing fails on missing evidence, so a 2.0.0 release with only
v0.2 provenance fails both tenets and the policy.

Treat prereleases differently from stable releases:

```hjson
code: "semver.prerelease(context.drop_version) == '' && size(predicates) > 0"
```

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
[CEL plugins reference](https://github.com/carabiner-dev/ampel/blob/main/docs/cel-plugins.md).

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

drop's own release policy, which branches on the version to accept the SLSA
v0.2 provenance of early prereleases while requiring v1 from stable releases,
lives in
[carabiner-dev/.github](https://github.com/carabiner-dev/.github/blob/main/ampel/policies/release/drop/drop-release.hjson).
