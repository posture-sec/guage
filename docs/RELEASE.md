# Release runbook

Task 4.5 asks for a release workflow with build provenance attestation and a verification command that is documented *and tested*, and it is the substance of gate G6. The owner chose, on 2026-11-03, to have this file instead of `.github/workflows/release.yml`. Two reasons, both recorded so nobody re-litigates them:

- `.github/` is a protected path under AGENTS.md section 12, so a new workflow is an owner decision rather than a contributor's convenience.
- An attestation workflow can only be proven by a tag, and cutting a release tag to prove the machinery that makes tags is backwards. D-12 already declines a tag until an auditing build exists, and of Phase 4's release preconditions 4.1, 4.4 and 4.6 are open alongside this task's own 4.5; 4.2 and 4.3 closed on 2026-11-05.

So 4.5 stays unticked and G6 stays unchecked. What this file buys is that the person cutting `v0.1.0` is following steps that have been run, not steps invented at the tag.

## 0. What is true today

`cmd/stellar-gauge/version.go` holds `var version = "0.1.0-dev"` with a comment naming task 4.8 as the change. There is no tag, no release, no artifact anywhere, and `README.md` says so in its own words: "There is no release to install." OSSF Scorecard scores `Packaging` and `Signed-Releases` at -1 on this repository, which is not a penalty to fix, it is the accurate reading of a project that publishes nothing.

## 1. Preconditions, in order

1. 4.4 green: the CI matrix builds and tests on Linux, macOS and Windows. Today only `linux/amd64` is built and tested in CI, so a release naming five platforms would ship three that no CI has ever run. Cross-compiling is not the same as testing; section 3 shows both facts separately.
2. 4.1 and 4.6 done, because 4.8 depends on 4.1 to 4.6 and G5 (Scorecard 8 or higher) is one of them. G5 is currently 6.8 and the gap is structural at one maintainer, not a repo change. 4.2 and 4.3, the usage docs and the README, closed on 2026-11-05 and are off this list.
3. The rule set is worth shipping. It was the reason this file was written: seven rules live and no signer family at all. That stopped being true on 2026-11-04, when task 3.1 registered the nine signer and threshold checks and 3.4 weighted them by role, and Phase 3 closed on 2026-11-05 with the facts report, so all sixteen v0.1 rules run and every Appendix B scenario has a golden behind it. What is still not settled is whether they are right about real accounts, which is 4.1 and gate G4 rather than D-12's judgement about a tag.

## 2. Build

Run from a clean checkout of the tag, not of a branch, because Go stamps the VCS revision into the binary and a release built from a dirty tree carries a revision that does not exist.

```
git checkout v0.1.0
go version                              # go1.27.3, which is D-10's pinned toolchain
```

One target per line, five targets total. `CGO_ENABLED=0` keeps the binary static and removes the C toolchain from the equation, which is what makes a macOS artifact buildable on Linux. `-trimpath` removes the build directory from the embedded paths. `-s -w` strips the symbol and DWARF tables. `-X main.version=$(VERSION)` overwrites the `0.1.0-dev` string, and `version` is the whole of it: `check` and `audit` print `ToolVersion` into both the text report and the JSON document, so an unstamped artifact reports itself as a development build forever.

```
VERSION=0.1.0
for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${t%/*}; arch=${t#*/}
  out=dist/stellar-gauge-$VERSION-$os-$arch
  mkdir -p "$out"
  ext=""; [ "$os" = windows ] && ext=.exe
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
    -trimpath -ldflags "-s -w -X main.version=$VERSION" \
    -o "$out/stellar-gauge$ext" ./cmd/stellar-gauge
done
```

Windows is `.zip`, everything else is `.tar.gz`, both because that is what the platform's users expect and because a `.exe` inside a tarball is a support ticket:

```
cd dist
for d in stellar-gauge-$VERSION-linux-* stellar-gauge-$VERSION-darwin-*; do
  (cd "$d" && tar -czf "../$d.tar.gz" .)
done
(cd "stellar-gauge-$VERSION-windows-amd64" && zip -qr "../stellar-gauge-$VERSION-windows-amd64.zip" .)
```

The archive contents matter as much as the binary: ship the license and a notice of the limits wording next to the executable inside the archive, since a downloaded tarball is detached from the repository that produced it.

### What was actually run on 2026-11-03

The loop above, with `VERSION=0.1.0-rc.1`, from this tree on `go1.27.3 linux/amd64`:

| target | result | stripped size |
|---|---|---|
| linux/amd64 | built | 7,086,240 |
| linux/arm64 | built | 6,553,760 |
| darwin/amd64 | built | 7,254,608 |
| darwin/arm64 | built | 6,708,258 |
| windows/amd64 | built | 7,298,048 |

`./dist/stellar-gauge-0.1.0-rc.1-linux-amd64/stellar-gauge version` printed `stellar-gauge 0.1.0-rc.1`, which is the `-X` stamp working. **Nothing in that table is a test**: no darwin or windows artifact was executed, because no runner here can execute them, and 4.4 is the task that changes that row from "built" to "green".

A rebuild of the same target into a different output directory produced SHA-256 `e86421cbc0b13c0b12d04e7e3f2cde759c190789e10541266abe60e0ae1d5db0` twice, byte for byte. That is the reproducibility the attestation in section 4 depends on, and it holds for this toolchain and this module set (standard library plus `github.com/BurntSushi/toml`, which has no transitive dependencies).

## 3. Checksums, and the verification command an operator runs

```
cd dist
sha256sum stellar-gauge-$VERSION-*.tar.gz stellar-gauge-$VERSION-*.zip > checksums.txt
```

A release is not done until `checksums.txt` is in the release body as text, not only as an attached file, so that the digest is compared against the same page the download came from. The verification command:

```
sha256sum -c checksums.txt
```

That line was run against the five `0.1.0-rc.1` archives above and returned `OK` for all five with exit 0. macOS and Windows users need the equivalent spelled out in the release notes, because `sha256sum` is not on macOS by default and not on Windows at all:

```
shasum -a 256 -c checksums.txt                                  # macOS
CertUtil -hashfile stellar-gauge-0.1.0-windows-amd64.zip SHA256  # Windows, manual compare
```

`CertUtil -hashfile` prints a digest, it does not verify against a list, so a Windows operator compares by eye. That asymmetry is worth one sentence in the release notes rather than a promise of parity.

Checksums prove the download is the artifact the release published. They prove nothing about where the artifact came from, which is the gap section 4 exists to close.

## 4. Build provenance attestation

Provenance is not a local operation. A GitHub build-provenance attestation is signed by an OIDC token issued to the workflow run, so it can only be produced by that run, and the corresponding `gh attestation verify` on a consumer machine only works because the signature resolves back to a repository, environment and run id that GitHub recorded. A runbook author cannot generate one by hand, which is the honest reason 4.5 is a workflow task and not a script.

The workflow 4.5 will add, as a design rather than as a merged file:

```yaml
name: release
on:
  push:
    tags: ['v*']
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write        # OIDC, for the attestation
      attestations: write    # storing the attestation
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - run: <the build and archive loop from section 2>
      - uses: actions/attest-build-provenance@<full SHA>      # resolve at authoring time
        with:
          subject-path: dist/stellar-gauge-*
      - uses: actions/upload-artifact@<full SHA>              # resolve at authoring time
        with:
          name: dist
          path: dist/
          digest: sha256
```

Two of those four pins are copied from `.github/workflows/ci.yml`, which is where this repository already runs them. The other two are `<full SHA>` on purpose: `attest-build-provenance` and `upload-artifact` are not used anywhere in this tree, so their SHAs were not read, and inventing a commit hash in a release runbook is worse than leaving it blank. Whoever opens 4.5 resolves both from the release being installed and records the version in the trailing comment, which is the convention the existing four workflows follow and the state Scorecard's `Pinned-Dependencies` measures: this repository scores 10 there, "8 of 8 GitHub-owned and 1 of 1 third-party actions pinned".

Three constraints the release workflow inherits, one of which is not yet a house rule:

- No workflow-level write permission, and narrow job-level permissions. All four existing workflows do this.
- `go-version-file: go.mod` rather than a hand-written version, because `go.mod` declares `go 1.27.3` and that is D-10's pinned toolchain.
- `persist-credentials: false` on checkout is what `scorecard.yml` sets, and the other three workflows do not set it. For a release job holding an OIDC token that mints a signed attestation, the safer answer is to set it. Say so deliberately rather than discovering it by copy-paste from `ci.yml`.

The verification command a user runs against an attested artifact:

```
gh attestation verify stellar-gauge-0.1.0-linux-amd64.tar.gz --owner posture-sec
```

What SLSA v1 build provenance proves: the bytes were produced by a specific workflow run in a specific repository at a specific commit. What it does not prove: that the source is trustworthy, that the binary behaves as documented, or that the tag was not moved after the run. It answers "did this artifact really come from that commit", and nothing more, so it belongs beside the checksum rather than instead of it.

## 5. Cutting the tag (task 4.8)

1. Confirm 4.1 to 4.6 are done in `ROADMAP.md`, and that task 4.5's own checkbox has its Done-when satisfied: a test release verified.
2. Set the version in `cmd/stellar-gauge/version.go` to `0.1.0` in the same pull request that removes the `0.1.0-dev` comment, so a checkout of `main` after the release is not stamped as a development build of the released version.
3. Build from the tag, not from `main`, per section 2.
4. Publish the release with the five archives, `checksums.txt` as text in the body, the verification commands from section 3 for all three platforms, and the limits paragraph from the report footer.
5. Re-read G1 to G10 as the task says, and record the Scorecard run on the tag commit rather than on `main`.
6. Task 6.2 repeats the whole of this for `v0.2.0`.

## 6. What remains unproven here

- No artifact was published and no tag was cut, so nothing in sections 4 and 5 has been executed. Section 2's build and section 3's checksum verification are the parts that were run, and their results are quoted above with the date.
- The provenance workflow is a design with unresolved pins. Its shape is derived from this repository's existing workflows and from the action's documented permissions, not from a run.
- The three non-Linux platforms are compiled and unexecuted until 4.4 lands.
- `Packaging` and `Signed-Releases` stay at -1, and G6 stays unchecked, and both are correct readings of a repository that has released nothing. Writing the runbook does not move either number, and this branch claims neither.
