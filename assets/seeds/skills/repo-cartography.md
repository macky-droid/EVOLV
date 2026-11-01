---
name: repo-cartography
description: Use when entering an unfamiliar repository or planning a large change across many files.
---
# Repo Cartography

You cannot fix a codebase you have not mapped. Map first, then read only the
parts the map points at.

## Pass 1 — shape (cheap)

1. `map_repo {path}` for the bounded tree and Go/JS/TS symbol outline. Read the
   `truncated` flag; if true, narrow to the subdirectory you need.
2. Read the root docs that actually exist: README, CONTRIBUTING, Makefile,
   Taskfile, go.mod/package.json/pyproject, CI workflows, Dockerfile.
3. Identify four things: entry point, core package(s), test command, and the
   generated/vendored directories to ignore.

## Pass 2 — paths (targeted)

Pick the two or three files on the path from input to the behavior you were asked
to change. Read them fully. Follow imports one hop, not the whole graph.

Useful questions:
- Where does the program start and how does it reach this feature?
- What is the data shape at the boundary?
- Where are errors produced and handled?
- What tests already cover this behavior?

## Pass 3 — plan

Write a short plan with set_plan:
- the files you expect to touch,
- the test command that will prove it,
- the risk (public API, data migration, platform-specific, UI contract).

Share a five-line map with the user before large changes: entry, core, tests,
risk, next step.

## Rules

- Do not paste whole files or long trees into chat. Summarize with paths.
- Do not read node_modules, dist, vendor, .git, or generated output.
- Do not infer architecture from filenames alone; open the file.
- Prefer `rg`-style search (grep) over reading every file when you have a symbol.
- If the repo is small (<20 files), skip the ceremony and just read it.

## Done when

You can name the entry point, the files on the path, the test command, and the
first concrete change — without guessing.
