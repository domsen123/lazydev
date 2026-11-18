---
name: lazydev
description: lazydev and its PRDs. Use this skill for lazydev, lazydev PRD, write a PRD, create a PRD, "make this a lazydev PRD", "turn this into a PRD", PRD erstellen, PRD schreiben, „mach daraus ein PRD“, files under prd/, the status line "> Status:", .lazydev/config.yaml, hooks, worktrees, runs, steps (create, implement, verify, review, merge, unblock), pull requests, starting and watching runs (Läufe starten, überwachen), resolving blockers (Blocker lösen), adding a project (Projekt hinzufügen) and questions about lazydev.
---

<!-- lazydev manages this file. The TUI start, "lazydev add" and "lazydev skill install" overwrite it. -->

# lazydev

lazydev implements PRDs (Product Requirements Documents) with Claude Code. A PRD describes a feature so
completely that an agent implements it without a human. lazydev implements each PRD in its own Git
worktree: plan → setup → implement → verify → review → merge, and fast-forwards it into the base branch
of the project. `verify` checks the feature in the running app and keeps evidence, only when
`.lazydev/verify.md` exists. With `merge: pr` in `config.yaml` the merge opens a pull request with `gh`
instead and the run waits in phase `pr` until a human merges it on GitHub.
You recognize a project by the folder `.lazydev/` with `config.yaml` and the prompts of the steps, the PRDs
live under `prd/<area>/<name>.md`.

Everything goes through the command line `lazydev`. The TUI (`lazydev` without arguments) is optional.
Never send the user to the TUI to press a key, use the matching command.

## Rules

- Write PRDs only under `prd/`. Implement nothing when the user wants a PRD.
- Change nothing under `.lazydev/`. The folder belongs to lazydev and is the same for the whole team.
- `language` in `.lazydev/config.yaml` decides the language of the prompts, the PRDs and the status
  values: `en` (default, also without the field) or `de`. Write a PRD in that language.
- Every PRD starts with `# PRD: <title>` and right after it the status line
  `> Status: <value> · Scope: <summary>`. Values with `en`: `planned`, `implemented (YYYY-MM-DD)`,
  `discarded (YYYY-MM-DD)`, `blocked (YYYY-MM-DD)`. With `de`: `geplant`, `umgesetzt (YYYY-MM-DD)`,
  `verworfen (YYYY-MM-DD)`, `blockiert (YYYY-MM-DD)`. A new PRD gets `planned` or `geplant`. lazydev
  reads both languages, older PRDs keep their values.
- Start, stop or discard a run only when the user wants it. `discard` deletes commits, ask first.
- `<project>` is optional: without it, the project of the current folder applies, also from a worktree.
  `<prd>` is the file name without `.md`.

## Tasks

| Task | How |
|---|---|
| Add a project to lazydev | `lazydev add [<path>]`, without path the repository of the current folder. Creates `.lazydev/`. |
| Write a PRD, also from the conversation so far | Run `lazydev prompt create "<summary in one sentence>"` in the project. The output is the complete create prompt in the language of the project. Follow it step by step, including the numbered clarifying questions with the options A to D, and wait for the answers. Without argument the task follows from the conversation. If the user should not be asked ("answer the questions yourself"), add `--auto`. |
| State of PRDs and runs | `lazydev status [<project>] [--json]`: phase, step, port, cost, errors and the questions of blocked runs. `--all` also shows done PRDs. |
| Start, continue, stop, discard a run | `lazydev start [<project>] <prd>`, `lazydev stop …`, `lazydev discard --yes …`. At most 3 runs work at the same time, more wait in the queue and start by themselves. `start` continues at the failed step after an error or abort. Just start new PRDs, also while others run: the first step `plan` decides itself whether the PRD runs in parallel or waits for others. |
| Change the order | `lazydev after [<project>] <prd> [<prd>…]` overrides the decision of `plan`, without PRDs it waits for nothing. `lazydev status` shows `waiting → <prds>` and the reason. |
| Watch runs | `lazydev wait [<project>] [--json] [--timeout 30m]` blocks until a run blocks, fails, aborts, gets merged, opens a pull request (`pr`) or gets a note, and prints the events. `idle` means: nothing works or waits, an open pull request waits for a human and does not count. In Claude Code the tool `Monitor` or a background command fits. |
| Log of a run | `lazydev log [<project>] <prd>`, `-f` follows it until the run ends. |
| See runs in Claude Code | `lazydev mod install` installs the pane `lazydev-runs` (queue, steps, cost, blockers), `/lazydev-runs` shows it. New sessions load it, a running one with `/reload-plugins`. |
| Let blockers resolve automatically | `lazydev auto-unblock [<project>] on` lets the runner resolve blocked runs itself (at most 2 times per run), `off` switches it off. Only when the user wants it. |
| Resolve a blocked run | `lazydev prompt unblock [<project>] <prd>` prints the unblock prompt, the PRD stands in it with its absolute path in the worktree. Follow it. With `--auto` you answer the questions yourself. Then `lazydev start <project> <prd>`. |
| Set the PRD status by hand | `lazydev set-status [<project>] <prd> planned\|merged\|discarded`, writes the value in the language of the project and commits the PRD. |
| Dev server in the worktree | `lazydev dev [<project>] <prd> [start\|stop]`, the port is in `lazydev status`. |
| Trust hooks or databases | `lazydev trust [<project>]` asks the user interactively. Ask them to run it themselves. |
| Questions about PRDs: format, status line, `## Blocker`, branch | Read `## PRDs` in `references/spec.md`. |
| Questions about `.lazydev/`: `config.yaml`, language, prompts, knowledge, trust | Read `## .lazydev/` in `references/spec.md`. |
| Questions about runs: steps, worktrees, merge, hooks, dev server, Discord | Read `## Workflow` and `## Processes` in `references/spec.md`. |
| Questions about the command line and files | Read `## CLI` and `## Data` in `references/spec.md`. |
| Questions about the TUI: panes, keys, git cockpit, menu | Read `## UI` in `references/spec.md`. |

`references/spec.md` is the complete specification of lazydev. Read only the section that fits the task,
not the whole file.

If `lazydev` is missing in `PATH`, tell the user to install lazydev
(https://github.com/domsen123/lazydev). Do not rebuild prompts and the PRD format from memory then, the
prompts belong to the project and to the version of lazydev.
