# Workflows

This repository ships a small CI/CD system, not a single YAML file. Every job
has a hard timeout, every long step has a shorter one, and no workflow waits on
a human approval or an unbounded loop.

## Map

| Workflow | Trigger | Purpose | Max runtime |
|---|---|---|---|
| `ci.yml` | push `main`, PR, merge group, manual | format, tidy, docs freshness, tests + coverage, real-server smoke on Linux + macOS, race, Go compat, cross-build, workflow/shell lint | 20 min/job |
| `release.yml` | `v*` tag, manual | validate, build 6 targets in parallel, smoke-test the binary, checksum, attest provenance, publish | 20 min/job |
| `nightly.yml` | daily 03:17 UTC, manual | repeated shuffled tests (`-count=3`), full race detector, cross-build | 35 min/job |
| `codeql.yml` | push `main`, weekly, manual | CodeQL static analysis for Go | 30 min |
| `security.yml` | push `main`, PR, weekly, manual | `govulncheck` against the live vulnerability database | 15 min |
| `dependency-review.yml` | pull request | fail on high-severity dependency changes | 10 min |
| `benchmark.yml` | weekly, manual | redaction benchmarks + step summary + artifact | 20 min |
| `installer.yml` | changes to `install.sh`, manual | bash syntax and `--help` on Linux + macOS | 10 min |

`ci.yml` uses the composite action in `.github/actions/setup-go/` so every job
installs Go from `go.mod`, enables the module/build cache, and prints the
toolchain it actually used.

## Guardrails against stuck runs

- **Every job has `timeout-minutes`.** Every test, build, and network step has
  a shorter `timeout-minutes` of its own.
- **Tests have a Go-level deadline**: `go test -timeout=10m` (15m for race,
  25m nightly), so a hung test is killed even if the runner is healthy.
- **Concurrency is explicit.** CI cancels superseded runs; release and nightly
  do not cancel each other's in-flight work.
- **No approval gates.** There is no `environment:` on the release job, so a
  release cannot sit forever waiting for a reviewer.
- **No `sleep`, `watch`, or polling loops.** Workflows are event-driven and
  finite.
- **Manual releases can be dry-run.** `release.yml` accepts `dry_run: true`;
  it validates and builds every artifact without creating a tag or release.
- **`GOTOOLCHAIN=local`** is set in CI, release, and nightly. A missing Go
  toolchain fails immediately instead of downloading an unexpected one.
- **Security and release use the latest `1.26.x` patch**, not the bare `go.mod`
  directive. A Go patch release can fix stdlib CVEs; `govulncheck` is run with
  that patched toolchain and the release binaries are built with it.
- **`GOFLAGS=-mod=readonly`** prevents CI from silently editing `go.mod` or
  `go.sum`; the separate `go mod tidy -diff` check reports drift.
- **Artifacts are verified before publish.** The release job checks all six
  binary names, runs `evolv --version`, generates checksums, and verifies
  them with `sha256sum -c` before attesting and uploading.
- **The smoke job boots the real binary.** It builds `./cmd/evolv`, starts it
  on a temporary home, probes `/api/health`, `/api/metrics` and the UI, then
  sends SIGTERM and fails if the process does not drain and exit.

## What runs on every PR

Required-in-practice checks:

1. `format / tidy / docs` — gofmt, `go mod tidy -diff`, generated docs current.
2. `test / coverage` — full suite, shuffled, coverage artifact.
3. `boot / api smoke` — build the binary, boot it on a temp home on Linux and macOS, probe health/metrics/UI, reject evil Origins/Hosts, and assert clean SIGTERM drain.
4. `race detector` — full module with `-race`.
5. `go 1.26.x` and `go stable` — compatibility.
6. `cross-build` — all six release targets.
7. `workflow / shell syntax` — YAML parses, `actionlint` passes, shell scripts parse.
8. `govulncheck` — known vulnerabilities in the module or stdlib fail.
9. `dependency review` — high-severity dependency changes fail.

CodeQL runs on `main` and weekly; it is not duplicated on every PR.

## Local equivalents

```bash
make check                                             # gofmt + vet + tests
make test                                              # tests only
make cross                                             # all six targets
go test -race ./... -count=1 -timeout=15m              # race job
go test ./... -count=1 -shuffle=on -timeout=10m        # test job
go test ./internal/redact -run '^$' -bench . -benchmem # benchmark job
```

Workflow YAML and scripts:

```bash
ruby -e 'require "yaml"; Dir[".github/**/*.{yml,yaml}"].each { |f| YAML.load_file(f); puts "ok #{f}" }'
bash -n install.sh
for f in scripts/*.sh; do bash -n "$f"; done
```

## Release procedure

1. Merge the change to `main`.
2. Create and push an annotated tag: `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
3. Watch the `release` workflow. It validates, builds the matrix, verifies the
   Linux binary's `--version`, writes `checksums.txt`, attests provenance, and
   publishes.
4. To rehearse without publishing, run the workflow manually with
   `dry_run: true`; the build artifacts are still uploaded for inspection.

## Troubleshooting a stuck run

- **Queued for a long time**: check the concurrency group. A manual release on
  `main` shares `release-refs/heads/main`; cancel the older run first.
- **A job is at its timeout**: open the job, find the last step, run that exact
  command locally. Do not increase the timeout until you know why it hangs.
- **`setup-go` is slow**: the module cache key is `go.sum`; if dependencies
  changed, the first run after the change must download. Retry once; if it
  persists, check GitHub status.
- **Tests hang only in CI**: compare `go version` from the setup step, and run
  with `-v -timeout=5m` locally. The `Diagnostics on failure` step prints the
  toolchain and cache paths on every failure.
- **Release fails after build**: inspect the `publish` job, not the matrix. The
  `Verify artifact set` step names any missing binary; `sha256sum -c` names any
  corrupt one.

## Action version policy

Actions are pinned to major tags (`@vN`) and updated weekly by Dependabot.
Major-version bumps are reviewed like code: read the action's release notes and
check input changes before merging.
