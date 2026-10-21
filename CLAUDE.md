# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

lazydev is a Go TUI (Bubble Tea) for the PRD workflow with Claude Code: projects left, terminal tabs
(PTY + x/vt) center with the details pane below, files and PRDs right. Each PRD runs in its own Git worktree
and port and gets fast-forwarded into the project's base branch (`baseBranch` in `.lazydev/config.yaml`,
default `main`). `SPEC.md` is the binding description of the behavior. To change behavior, update `SPEC.md`,
`README.md` and `README.de.md` along with the code.

## Commands

```sh
make test                                   # go vet ./... and go test ./...
make build                                  # bin/lazydev
make install                                # ~/.local/bin/lazydev, PREFIX overridable
go run ./cmd/lazydev                        # TUI without installing
go test ./internal/runner -run TestRunMerges  # single test
```

Linux only: lazydev uses `/proc`, `setsid` and `flock`. Go 1.27 or later. `x/cellbuf` is pinned to v0.0.15,
older versions do not build against the newer `x/ansi`.

## Architecture

### Packages (`internal/`)

| Package | Role |
|---|---|
| `tui` | Model, layout, keys, panes, wizard, status picker, run view, docker view. Reads files only |
| `terminal` | Programs in a PTY, x/vt emulator, key encoding, window title filter |
| `runner` | Start/queue/stop/discard, steps setup → implement → review → merge, hooks, worktrees, dev server start |
| `state` | Run state, step logs, locks, trust, paths under `$XDG_STATE_HOME/lazydev/` |
| `project` | PRDs, status line, phases, status change with commit |
| `repo` | `.lazydev/`: config, init with the simple workflow (`workflows/simple/`), trust hash, wizard YAML edits |
| `prompts` | Prompt = header + knowledge + `.lazydev/<step>.md` + built-in contract (`contract/`) |
| `claude` | `claude -p` stream-json, summary, log descriptions, built-in agents (`agents/`) |
| `config` | `projects.json`, `settings.json`, project IDs, path helpers |
| `docker`, `devserver`, `usage`, `git`, `fsutil` | Containers, detached dev servers, Claude limits, git binary, atomic writes |

### Process model

- The TUI controls nothing directly. It reads run state and logs every second and PRDs and git info every
  5 s, because an ending runner causes no event.
- Each PRD runs in a detached runner `lazydev run <id> <dir> <prd>` (`runner.spawn`, `setsid`). The binary
  gets copied to `$XDG_STATE_HOME/lazydev/bin/lazydev` first, so a running runner keeps its code and `go run`
  works. `state.IsRunner` checks `/proc/<pid>/cmdline`.
- `runner.MaxParallel` (3) holds under the lock `locks/start.lock`. An ending runner starts the next waiting
  PRD, not the TUI.
- `x` signals the runner only. It cancels its context first, then signals its process group, so the step
  records an abort instead of a killed Claude.
- `create` is no runner step: `n` opens an interactive Claude Code tab with the create prompt.

### Runner (`internal/runner`)

Every step lands in `steps` of the run state and must be repeatable, `s` continues at the failed step.

- `Run` checks trust of `hooks`/`allowedTools` first (`repo.Config.TrustHash`, `state.Trusted`).
- Claude runs as `claude -p … --output-format stream-json --verbose` with stdin `/dev/null` and
  `GIT_EDITOR=true`. All permissions come through `--allowedTools`, new worktrees have no trust dialog.
- A Claude step succeeds only with exit code 0, a last `result` without error and its check: after
  implement `## Ergebnis` and the PRD status line, after review a clean worktree. After implement, review
  and the Claude step merge, `checkLazydevUntouched` makes sure nothing under `.lazydev/` changed.
- Merge (`merge.go`): `flock` per project, `git rebase <base>`, on conflict `rebase --abort` and the Claude
  step merge, check hook, cost into the PRD, wait for a clean main folder, `git merge --ff-only`, cleanup.
  Merges always run right after the review.
- Hooks are scripts from `config.yaml` only, there are no presets.

### Paths

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` and `XDG_DATA_HOME` apply. Projects in `~/.config/lazydev/`, state and
logs in `~/.local/state/lazydev/runs/<id>/`, worktrees in `~/.local/share/lazydev/worktrees/<id>/<prd>/`.

## Tests

- Tests isolate via `t.Setenv("XDG_STATE_HOME", t.TempDir())`, `XDG_DATA_HOME` and `XDG_CONFIG_HOME`. Git
  tests set `GIT_AUTHOR_*` and `GIT_COMMITTER_*`.
- Runner tests run `runner.Run` in the foreground against temporary repos with a fake `claude` script
  (`claude.Command`). It recognizes the step by the header `# lazydev <step>:` in `$2`. To test new runner
  scenarios, extend that script. No test spawns a detached runner.
- Tests with hooks or allowed tools in `.lazydev/config.yaml` trust the hash with `state.Trust`.

## Conventions

- All texts are English: UI strings, errors, comments, docs. Exceptions: `README.de.md` is the German version
  of `README.md` with the same content, always change both. Prompts, contracts, agents and PRD status values
  (`geplant`, `umgesetzt`, `verworfen`, `Kosten:`, `Modell:`) stay German, existing PRDs and prompts use them.
- Commits follow Conventional Commits in English, e.g. `feat(tui): show keys for focus and state`.
- Commits lazydev makes itself use `git commit` without Claude and without trailer.
