# drop

A secure-first installer for GitHub releases.

`drop` downloads a binary, archive or package from a project's GitHub
release, verifies it against the security policies its publisher defines
before anything touches your system, and installs it. Policies are evaluated
with the [AMPEL](https://github.com/carabiner-dev/ampel) policy engine against
the attestations shipped with the release: SLSA provenance, SBOMs, signatures.
No policy, no install.

```
$ drop install carabiner-dev/drop
  💫 Looking for policies (source: https://github.com/carabiner-dev/.github ampel/policies/release/{_,drop}/)
      ✔️  1 policy sets found
  ⏬ Downloading drop-v0.0.1-linux-amd64 (57.18 MB)
  🛡️  Verifying artifact...
      ✅  PASS: SLSA provenance attestation found
      ✅  PASS: Authorized builder ID detected
      ✅  PASS: Expected buildType found: https://actions.github.io/buildtypes/workflow/v1
      ✅  PASS: Expected build point found: git+ssh://github.com/carabiner-dev/drop
      ✅  PASS: Stable release 0.0.1 ships SLSA v1 provenance
  🔧 Installing binary to /usr/local/bin/drop...
  🎉 drop installed!
```

## Installing drop

Grab a binary from the [releases page](https://github.com/carabiner-dev/drop/releases)
or build it with Go:

```bash
go install github.com/carabiner-dev/drop@latest
```

Once you have it, drop can keep itself up to date like any other app:
`drop install carabiner-dev/drop` verifies the release against
[drop's own policy](https://github.com/carabiner-dev/.github/blob/main/ampel/policies/release/drop/drop-release.hjson).

## Usage

Apps are named by repository, with an optional tag and asset name:

```bash
drop install github.com/org/repo          # latest stable release
drop install org/repo@v1.2.3              # a specific tag
drop install org/repo#toolname            # when the binary is not named after the repo
```

### Install

```bash
drop install org/repo
```

drop lists the release assets, groups them into installables (one binary
shipped for several platforms is one installable), picks the variant for your
OS and architecture, verifies it and installs it. Bare binaries and binaries
inside archives (`tar.gz`, `zip`, `tar.xz`, `tar.zst`, `tar.bz2`) go to
`--bin-dir` (`/usr/local/bin` by default, through `sudo` when needed).
Packages (`rpm`, `deb`, `apk`) are installed with the system's package
manager. When a release offers both, drop asks, or keeps the package manager
in charge if the app is already installed as a package. `--type` forces a
choice, `--entry` names the executable to take from an archive, and `--yes`
takes the defaults for scripts and CI.

Installs are recorded, so a repeat `drop install` is refused. Use `drop update`
to upgrade, or `--force` to reinstall, switching formats if `--type` says so:
the previous binary or package is removed once the new one is in place.

### Listing installed apps

When an app is installed, drop registers it in a catalog in the user's directory.
This allows drop to keep track of the installed software to update and remove it.

```bash
drop list                # what is installed, where, and whether it was verified
```

```text
    NAME         VERSION   KIND      LOCATION                    VERIFIED   UPDATED      REPOSITORY
   ─────────────────────────────────────────────────────────────────────────────────────────────────────────────
    atomscan     v2.11.0   archive   /usr/local/bin/atomscan        ✘       2026-09-14   atomdrift-project/scan
    drop          v0.0.1   binary    /usr/local/bin/drop            ✔       2026-09-14   carabiner-dev/drop
    goreleaser   v2.18.1   archive   /usr/local/bin/goreleaser      ✘       2026-09-14   goreleaser/goreleaser
    scorecard     v5.5.0   archive   /usr/local/bin/scorecard       ✘       2026-09-07   ossf/scorecard
    cosign        v3.1.2   binary    /usr/local/bin/cosign          ✘       2026-07-19   sigstore/cosign
    gitsign      v0.16.1   package   rpm package                    ✘       2026-07-19   sigstore/gitsign

  6 app(s) installed with drop
```

### Updating installed apps

```bash
drop check-update        # which installed apps have newer stable releases
drop update              # reinstall them the way they were installed, after verifying
```

### Download

```bash
drop get org/repo                      # the installable for your platform, verified, into the current directory
drop get org/repo -p darwin/arm64      # another platform
drop get org/repo#checksums.txt        # a specific asset
```

### Explore a Repository

If you want to see the artifacts a repository releases, use the `ls` subcommand

```bash
drop ls org/repo          # installables in the latest release
drop ls -r org/repo       # list available releases
drop ls -l --all org/repo # every asset, long format
```

Example run: `ls -l sigstore/cosign`:

```text
total 4
💾🐧🍏🪟📦➖➖➖  sigstore-bot  sigstore  52763563  Aug    6    19:06  cosign
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  3906      Aug    6    19:06  cosign_checksums.txt
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  6406      Aug    6    19:06  cosign_checksums.txt.sigstore.json
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  178       Aug    6    19:06  release-cosign.pub
```

You can see from the legend in the first line that `cosign` is offered:

- For Linux (🐧), Mac(🍏), and Windows (🪟)
- And as a binary (💾) and some sort of installable package (📦)

The ourput also shows when it was released and by who.

## Verification

Every artifact is verified before it is installed or written to disk. drop
reads the policies a project's organization publishes in its `.github`
repository, under `ampel/policies/release/<repo>/` for the repository and
`ampel/policies/release/_/` for policies that apply to every release in the
organization.

Projects without policies of their own fall back to the community policies in
[policylabs/oss](https://github.com/policylabs/oss) (under construction).

Policies are written in [HJSON](https://hjson.github.io/) or JSON, and can
reference the shared library in
[carabiner-dev/policies](https://github.com/carabiner-dev/policies).

Policies can declare which releases they apply to, so a project whose security
metadata changed over time holds each release to what it shipped. `drop` exposes
the version being installed to the AMPEL verifier so policies can gate on versions.
How that works, what drop injects, and how to test a policy before publishing it
is in [docs/policies.md](docs/policies.md).

When a project has no policies, drop stops and says so. You can then:

```bash
drop request org/repo                        # open an issue asking for community policies
drop install --policy-repo ./my-policies org/repo   # use your own policy source (a repo or a local checkout)
drop install --insecure org/repo             # skip verification, you are on your own
```

### Attesting the Verification

`--attest` writes an attestation of the verification next to the download, as
the full AMPEL result set, a SLSA
[Verification Summary](https://slsa.dev/spec/v1.1/verification_summary) or an
in-toto Simple Verification Result, signed with sigstore by default.

## Building drop from Source

```bash
go build ./...
go test ./...
```

## Contributing

Drop is Open Source software, developed by Carabiner Systems, Inc and released
under the [Apache 2.0](LICENSE) license terms. Feel free to use and modify it
and, most importantly, contribute to it sending patches or opening issues, we
love feedback!
