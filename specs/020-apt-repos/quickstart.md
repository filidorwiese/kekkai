# Quickstart: validating `image.apt_repos` end-to-end

Constitution IV: validation is e2e against a real docker daemon. Build the binary, run in a scratch project. `kekkai up` from a non-TTY exits after the build with "the input device is not a TTY" — that is fine for every scenario here except S1's in-sandbox check, which uses the pseudo-TTY pattern.

## Prerequisites

```sh
go build -o kekkai-test ./cmd/kekkai        # from repo root
mkdir -p /tmp/kekkai-aptrepos && cd /tmp/kekkai-aptrepos
```

Cleanup after each sandbox run: `docker rm -f <container>` if needed, and remove only this project's `kekkai-history-<hash>` volume. Expect builds of ~2–4 min (apt + node + claude).

## S1 — Third-party repo happy path (US1; SC-001)

`.kekkai.yaml`:

```yaml
image:
  apt_repos:
    - name: dart
      url: https://storage.googleapis.com/download.dartlang.org/linux/debian
      suite: stable
      components: main
      key_url: https://dl-ssl.google.com/linux/linux_signing_key.pub
  apt_packages: [dart]
```

Run `../path/to/kekkai-test up` (non-TTY). Expect: image builds without error (key fetched, repo registered, dart installed). Then start a live sandbox via pseudo-TTY (`script -qec "kekkai-test up" /dev/null </dev/null &`), poll `docker ps`, and check `kekkai-test exec dart --version` prints a Dart version. Also `kekkai-test exec cat /etc/apt/sources.list.d/kekkai-dart.list` shows exactly the contracted deb line (see [contracts/image-render.md](contracts/image-render.md)).

## S2 — No key_url, system keyrings (US1 scenario 3)

Replace config: repo `name: backports`, `url: https://deb.debian.org/debian`, `suite: trixie-backports` (no `key_url`), `apt_packages: []`. Run `up`. Expect: build succeeds; in-sandbox `apt-cache policy` (via `kekkai-test exec`) lists the backports source; the sources line has `arch=` but no `signed-by`.

## S3 — Validation rejections, one pass (US2; SC-002)

Config with several bad entries at once — e.g. `name: Bad_Name`, a duplicate `name`, `url: http://...`, `suite: "stable [trusted=yes]"`, a flat entry (`suite: ./`) with `components: main`, and `name: github-cli`. Run `up`. Expect: `invalid configuration (N violation(s))` listing **all** violations, each naming `image.apt_repos[<i>]` and the field, per [contracts/config-validation.md](contracts/config-validation.md); no docker work started (no build output).

## S4 — Rebuild on change, reuse on no-change (US3; SC-003)

With the S1 config already built: run `up` again → no "building image" line (tag reused). Edit `suite: stable` → any other valid value or add a second repo → run `up` → new tag, rebuild. Revert → original tag reused without rebuild. Also verify zero-repo neutrality: a config **without** `apt_repos` must reuse the same image tag it used before this feature's binary (byte-identical render contract).

## S5 — Signature-failure hint (US4; SC-004)

S1 config minus `key_url`. Run `up`. Expect: build fails at `apt-get update` with apt's NO_PUBKEY/"is not signed" error, and stderr additionally shows the hint naming `image.apt_repos[0] (dart)` and suggesting `key_url` (exact wording in [contracts/image-render.md](contracts/image-render.md)). Exit status non-zero.

## S6 — Firewall unaffected (SC-005)

During the S1 live sandbox: firewall init lines unchanged, and the repo/key hosts are **not** reachable from inside (`kekkai-test exec curl -sI --max-time 5 https://storage.googleapis.com` fails) unless the user allows them via `network.*`. Never add `example.com` to allowed_domains in tests (it is the must-be-blocked probe).

## S7 — Docs (FR-011)

`kekkai-test init` in an empty dir: starter contains the commented `apt_repos` block. README image section and SPECIFICATION.md §4.2/§4.4/§6.1/§6.3 mention the fields (grep `apt_repos`).
