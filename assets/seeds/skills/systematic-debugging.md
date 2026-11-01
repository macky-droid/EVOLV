---
name: systematic-debugging
description: Use when a build, test, or runtime behavior fails and the cause is not obvious.
---
# Systematic Debugging

Debugging is a search problem, not a guessing contest. Change one variable at a
time and let evidence choose the next step.

## Procedure

1. **Reproduce on purpose.** Find the exact command, input, or click path that
   fails. Run it yourself. If it does not reproduce, say so and gather the
   missing condition (environment, version, data) before changing code.
2. **Read the whole error.** The first line names the failure; the last lines
   often name the cause. Read the stack, the file, and the line — not just the
   word "error".
3. **Narrow the surface.** Use map_repo to find the entry point, then read the
   smallest path from input to failure. Add temporary logging only when reading
   is not enough; remove it before finishing.
4. **State one hypothesis.** Write it as: "X fails because Y, which I can test
   by Z." If you cannot name Z, you are not ready to edit.
5. **Test the hypothesis cheaply.** A focused test, a one-line script, or a
   single log line beats a broad rewrite. Do not change three things at once.
6. **Fix the root cause.** If the fix is a workaround, say what the real cause
   is and why the workaround is acceptable now.
7. **Prove it.** Re-run the original reproduction, then the full relevant suite.
   Add a regression test that fails without the fix and passes with it.
8. **Report the chain.** What failed, what caused it, what changed, what you ran.

## Traps

- Editing before reproducing; the bug moves and you lose the baseline.
- Trusting a cached/stale test result — rerun with -count=1 where available.
- Fixing the symptom (null check) instead of the source (why was it null?).
- Changing the test to match the buggy output.
- Debugging by rewriting: large diffs hide the actual change.
- Forgetting the second failure behind the first one; rerun after each fix.

## Done when

The original reproduction passes, the regression test fails on the old code, the
relevant suite is green, and the explanation fits every observed symptom.
