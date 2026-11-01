---
name: self-evolution-playbook
description: Use before rewriting your own body, tools, prompts, plugins, or kernel.
---
# Self-Evolution Playbook

Changing yourself is normal work. Changing yourself blindly is how an organism
loses an organ. This playbook is the safe sequence.

## 1. Decide what kind of growth it is

| Need | Action | Lives in |
|---|---|---|
| A visible panel, timer, chart, game | evolve_component | workspace/ui/components/ |
| Reusable logic you will call again | evolve_tool + schema | workspace/tools/ |
| A procedure future turns must follow | evolve_skill | workspace/skills/ |
| Durable identity or preference | evolve_prompt | workspace/prompts/ |
| Reaction to events or a live timer | evolve_plugin | workspace/plugins/ |
| A change to the launcher/kernel | stage_candidate or self_modify | cmd/, internal/ |

Do not use a prompt fragment for a procedure, or a component for logic. Choose
the smallest organ that does the job.

## 2. Inspect before writing

Read the current file first. `read_file` returns the hash you need. Check what is
already mounted or grown with list_dir, list_tools, and the current prompt index.
Never overwrite a file you have not read unless you are creating a new one.

## 3. Freeze a restore point

Call snapshot before broad rewrites, plugin replacement, prompt surgery, or any
kernel edit. Snapshots are cheap; regret is not.

## 4. Mutate in one deliberate batch

Send complete file contents for files you own. Use expected_hash for existing
files. Keep the batch focused: one logical change, plus the reads it needs. Do
not mix an unrelated restyle into a bug fix.

## 5. Verify by reading back

After a write, read the file or inspect the result. Components: check the HTML
is self-contained and IDs are prefixed. Tools: call run_tool with a real example
and with a bad argument to prove the schema rejects it. Plugins: check the load
log and one real event. Prompts: confirm the fragment is present next turn.

## 6. Kernel changes are surgery

1. `snapshot` first.
2. Read the exact Go files; keep the diff small.
3. Prefer `stage_candidate` with expected hashes: it builds and tests an isolated
   copy and never touches the living source.
4. If you must use `self_modify`, use `stage_only:true` until green, then ask the
   user before swapping the launcher.
5. Never promote a candidate without explicit user approval (`confirm:true`).
6. After a swap, the running process is still the old body — tell the user to
   restart `evolv`.

## 7. Roll back without drama

If a mutation breaks the shell or the kernel, `rollback {"path":"latest"}`
restores the genome. For a bad candidate, leave it staged and report the failed
check; do not promote it. If the browser is broken but the kernel works, write a
valid shell file and reload_shell.

## 8. Close the loop

Update the plan, call complete_task with files and test counts, and tell the user
in one sentence what grew. If you evolved a reusable procedure, evolve_skill it.

## Anti-patterns

- Rewriting a whole file from memory instead of reading it.
- Ten tiny batches where one batch would do.
- Re-evolving a plugin and leaving the old sandbox running (the kernel replaces
  it for you; verify with one event).
- Promoting a candidate because it compiled, without the user's approval.
- Editing the kernel to make a test pass by weakening the test.
- Leaving debug prints, dead files, or a stale plan behind.
