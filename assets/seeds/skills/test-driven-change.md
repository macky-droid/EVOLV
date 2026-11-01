---
name: test-driven-change
description: Use when fixing a bug or adding behavior that can be checked automatically.
---
# Test-Driven Change

A test is how a future turn knows you were right. Write the check before the
change when the behavior is testable; keep it after the change when it is not.

## Loop

1. **Find the harness.** Look for go.mod + *_test.go, package.json, pyproject,
   or Cargo.toml. Prefer `run_tests` — it detects the framework and returns
   counted results. If no harness exists, add the smallest one that runs.
2. **Write the failing check.** For a bug, reproduce the exact broken behavior.
   For a feature, assert the smallest user-visible contract.
3. **Run it and watch it fail for the right reason.** A test that fails because
   it does not compile proves nothing yet.
4. **Make it pass with the smallest change.** No opportunistic refactors in the
   same step; they hide the fix.
5. **Refactor under green.** Clean names, remove duplication, keep the test.
6. **Run the full relevant suite.** Report pass/fail/skip counts honestly.

## Rules

- Test behavior, not implementation details. Assert the contract a caller sees.
- One behavior per test; a descriptive name beats a comment.
- Do not mock what you can run for real in the same process.
- Table-driven tests are good when the cases share setup; do not force them.
- Never delete, skip, or loosen a failing assertion to get green.
- Never claim "tests pass" without the command output. If tests cannot run
  (missing runtime, network, platform), say exactly why and what you did check.
- Flaky tests: fix the shared state or the timing assumption; do not retry-loop.

## Commands this workspace knows

- Go: `go test ./...` (add `-count=1` when caching is suspicious)
- Node: `npm test` (or the script named in package.json)
- Python: `pytest`
- Rust: `cargo test`

Use `run_tests` first; fall back to `shell` only when the project needs a custom
command or environment.

## Done when

The new check fails before the fix, passes after it, the surrounding suite is
green, and the user can run the exact command you ran.
