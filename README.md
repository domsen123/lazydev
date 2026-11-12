# lazydev

<p align="center">
  <b>English</b> · <a href="README.de.md">Deutsch</a>
</p>

PRD workflow for Claude Code. You write a PRD with Claude, lazydev implements, reviews and merges it without
you: every PRD in its own Git worktree with its own port, fast-forwarded into the project's base branch.
You drive it from any Claude Code session, from the shell or from the optional terminal UI.
[SPEC.md](SPEC.md) describes the behavior in full.

## How it works

```
prd/<area>/<name>.md  (geplant)
  │  lazydev start
  ▼
plan ─► setup ─► implement ─► review ─► merge  ─►  base branch  (umgesetzt)
               │                         │
               └──── blocked: open questions ────► you or Claude answer ─► continue
```

| Step | Does |
|---|---|
| plan | Only with other open runs in the project: Claude reads their PRDs and the code, decides whether the PRD runs in parallel or waits until some are merged. Read-only |
| setup | Worktree on `feat/<prd>`, untracked PRD moved and committed, databases, setup hook |
| implement | Claude implements the PRD. Success: `## Ergebnis` `umgesetzt` and status `umgesetzt (date)` |
| review | Claude reviews and commits. Worktree must be clean afterwards |
| merge | Rebase on the base branch (Claude on conflicts), check hook (Claude on red), cost into the status line, `git merge --ff-only`, teardown hook, cleanup |

- No human between start and merge. Nothing reaches the base branch without a green check hook.
- Max 3 runs at a time, the rest waits in a queue. Ports from 3500.
- Every step is repeatable: after an error or a stop, `lazydev start` continues at the failed step.
- Claude may not touch `.lazydev/`. Prompts, PRD status values and the reports Claude writes stay German.

## Requirements

- Linux (`/proc`, `setsid`, `flock`), Go 1.27+ to build.
- `git` and [Claude Code](https://claude.com/claude-code) (`claude`) in `PATH`.
- Optional: `docker` for databases per worktree and the Docker tab, `$VISUAL`/`$EDITOR` for opening files.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/domsen123/lazydev/main/install.sh | sh
sh install.sh v0.2.0  # a specific release instead of the newest
lazydev update        # later: install the newest release over the binary
```

`install.sh` needs `curl`, `tar`, `sha256sum` and `uname`, checks the checksum and installs to
`$PREFIX/bin` (`PREFIX` defaults to `~/.local`). From source: `make install` or `go run ./cmd/lazydev`.
Running runners keep the code they started with.

## Get started

```sh
cd ~/code/shop
lazydev add            # adds the project, creates .lazydev/, installs the Claude Code skill
lazydev mod install    # optional: pane with the runs beside the Claude Code chat
git add .lazydev && git commit -m "chore: add lazydev"
```

Then, in a Claude Code session in the project:

1. Discuss the feature, then say "make this a lazydev PRD". Claude asks numbered questions with options A to
   D and writes `prd/<area>/<name>.md` with status `geplant`.
2. Say "start it". Claude runs `lazydev start <prd>` and watches the run with `lazydev wait`.
3. If the run blocks, Claude asks you the open questions, writes the answers into the PRD and continues.
4. After the merge the PRD says `umgesetzt (date) · Kosten: … · Modell: …` on the base branch.

The same from the shell:

```sh
lazydev start login-form   # runs in the background, no session needed
lazydev status             # phase, step, port, cost, blocker questions
lazydev log login-form -f  # follow the step logs
lazydev wait               # block until a run blocks, fails, aborts or merges
```

## PRDs

`prd/<area>/<name>.md`, status line within the first 20 lines:

```markdown
# PRD: Login form

> Status: geplant · Scope: …
```

| Status | Meaning |
|---|---|
| `geplant` | planned, can start |
| `blockiert (date)` | run waits for answers, see `## Blocker` |
| `umgesetzt (date) · Kosten: x.xx USD · Modell: …` | merged |
| `verworfen (date)` | discarded |

Sections lazydev and Claude add: `## Blocker` (open questions of a blocked run), `## Entscheidungen`
(questions Claude answered itself, with option and reason), `## Hinweise` in the implement report (remarks
that do not block). `lazydev set-status <prd> planned|merged|discarded` changes the status by hand and
commits a tracked PRD.

## Runs

### Order and parallelism

With other open runs in the project, plan decides whether the PRD may start next to them. A held-back PRD
waits without taking a slot and starts once the PRDs it waits for are merged or discarded. `lazydev status`
shows `waiting → <prds>` and the reason.

```sh
lazydev after login-form user-model   # override plan: start only after user-model is merged
lazydev after login-form              # wait for nothing
```

### Stop, continue, discard

```sh
lazydev stop <prd>             # the step records an abort
lazydev start <prd>            # continues at the failed, blocked or next step
lazydev discard [--yes] <prd>  # worktree, branch and commits gone, PRD back to geplant
```

The merge waits while the main folder is not on the base branch or has changes outside `prd/` and
`.lazydev/`. If the base branch moved meanwhile, it rebases again.

### Blocked runs

When the PRD leaves a decision open, Claude aborts implement or the merge with open questions. That is no
error: the run ends in phase `blocked`, the PRD in the worktree gets `blockiert (date)` and `## Blocker`.

- **In Claude Code:** the skill runs `lazydev prompt unblock <prd>`, asks you the questions, works the
  answers into the PRD and runs `lazydev start <prd>`. Say "answer the questions yourself" for `--auto`.
- **Auto-unblock** (`lazydev auto-unblock on`, per project): the runner lets Claude answer the questions
  itself in the worktree, records them under `## Entscheidungen` and runs the blocked step again, at most 2
  times per run. Questions about data loss, security, public interfaces or costs it may not decide, then the
  run stays blocked.
- `lazydev start` refuses while the PRD still says `blockiert`.

## Claude Code integration

### Skill

`~/.claude/skills/lazydev/`, written by `lazydev add`, `lazydev skill install` and every TUI start, so it
follows each update. A normal Claude Code session then knows lazydev and its CLI: it writes PRDs with
`lazydev prompt create` (the same prompt and questions as `n` in the TUI), starts, stops and watches runs,
resolves blockers and answers questions about lazydev from the bundled spec. It never sends you to the TUI.
`"skill": false` in `settings.json` stops the automatic write; to remove, delete the folder too.

### Mod

`lazydev mod install` adds a pane beside the Claude Code chat, like the agents panel: cost, running and
waiting runs, then each running PRD with phase, model, port, `3/5 · implement` with progress bar, cost and
time, blocked runs with their questions, the queue with what each PRD waits for, and the planned PRDs. It
opens once a run works, waits or is blocked; `/lazydev-runs` toggles it. It reads
`lazydev status --here --json` every 3 s, so it shows the project of the session's folder only. Lives in
`~/.claude/skills/lazydev-runs/`, `lazydev mod uninstall` removes it, `/reload-plugins` loads it into a running
session.

## `.lazydev/`

Versioned in the project, the same for everyone. `lazydev add` creates missing files, never overwrites.

| File | Content |
|---|---|
| `config.yaml` | `baseBranch`, `knowledge`, `allowedTools`, `hooks`, `databases` |
| `create.md`, `implement.md`, `review.md`, `merge.md`, `unblock.md` | Prompts of the steps |

```yaml
baseBranch: develop          # default main
knowledge:                   # files Claude reads first: all, create, implement, review, merge, unblock
  all: [CLAUDE.md]
  implement: [docs/architecture.md]
allowedTools:                # on top of the built-in tools of each step
  - Bash(pnpm test:*)
hooks:                       # scripts, relative to the main folder
  setup: scripts/lazydev-setup.sh
  check: scripts/lazydev-check.sh
  teardown: scripts/lazydev-teardown.sh
  dev: scripts/lazydev-dev.sh
```

- Prompt = header + knowledge block + `.lazydev/<step>.md` + built-in contract. The contract cannot be
  changed per project.
- Hooks get `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`, `LAZYDEV_SLUG`. The
  dev server (`lazydev dev <prd>`) also gets `PORT`.
- Hooks, `allowedTools` and `databases` need trust: `lazydev trust`. Changing them asks again.

### Databases per worktree

For projects with Postgres in Docker Compose, `databases` gives every worktree its own empty database
`<name>_<slug>`. The TUI wizard (`i`) finds Postgres services in the compose file and writes the section.

```yaml
databases:
  - service: db                        # compose service
    env:
      file: .env                       # relative to the main folder, must be ignored by git
      key: DATABASE_URL
  - service: db-test
    compose: docker-compose.dev.yml    # without it the default compose.yaml or docker-compose.yml
    env:
      file: .env.test
      key: NUXT_DATABASE_NAME
```

- `DATABASE_URL="postgres://app:secret@localhost:5432/app?sslmode=disable"` and slug `login_form` give the
  worktree `…/app_login_form?sslmode=disable`. A plain value like `app_test` becomes `app_test_login_form`.
- Setup copies the env files into the worktree, starts a stopped service, creates the databases, then runs
  the setup hook. User is `POSTGRES_USER` of the service, else `postgres`, everything via `docker exec`.
- If the base branch moved before the merge, the databases and the setup hook run again before the check.
- Merge and discard drop them after the teardown hook, only names ending in `_<slug>`.

Migrations and seeds stay in the setup hook:

```sh
#!/bin/sh
# scripts/lazydev-setup.sh: the worktree's .env already names its own database.
set -e
pnpm install --frozen-lockfile --prefer-offline
pnpm db:migrate
```

### Global settings

`~/.config/lazydev/settings.json` (rights `0600`), applies to every Claude step of all projects:

```json
{
  "permissionMode": "auto",
  "allowedTools": ["Bash(head:*)", "Bash(tail:*)", "Bash(sed -n:*)", "Bash(grep:*)"]
}
```

`permissionMode` is `acceptEdits` (default) or `auto`; any other value fails the step before Claude starts.
In `auto` a classifier of Claude Code decides each tool call, not deterministic. `allowedTools` here need no
trust and come before those of `config.yaml`.

## Discord

Runners post to a Discord webhook when a PRD is merged, a step fails or blocks, Claude resolves a blocker, a
run is stopped or the merge waits for the main folder. No TUI needed.

```json
{
  "discord": {
    "enabled": true,
    "url": "https://discord.com/api/webhooks/…",
    "projects": {
      "/home/me/code/shop": {"enabled": false},
      "/home/me/code/blog": {"url": "https://discord.com/api/webhooks/…"}
    }
  }
}
```

The URL lives in `settings.json` only, never in a repository. The TUI menu (`alt+m` → `Discord`) sets it too
and has `Send test`. Each message is an embed with project, branch, cost, duration and model; `⊘ Blocked`
adds the questions. Failed deliveries retry 3 times 5 s apart, then the step log notes it, the run goes on.

## CLI

```sh
lazydev add [<path>]                              # add a project, without path the repo of the current folder
lazydev remove <project>                          # remove it from the list, the folder stays
lazydev projects [--json]
lazydev trust [<project>]
lazydev auto-unblock [<project>] [on|off]         # let Claude resolve blocked runs of the project itself
lazydev status [<project>] [--all] [--here] [--json]  # PRDs with phase, step, port, cost and blocker questions
lazydev start [<project>] <prd>                   # start or continue in the background
lazydev stop [<project>] <prd>
lazydev discard [--yes] [<project>] <prd>
lazydev wait [<project>] [--json] [--timeout 30m] # block until a run gets blocked, fails, aborts or merges
lazydev log [<project>] <prd> [-f]                # step logs of a run, -f follows them
lazydev set-status [<project>] <prd> planned|merged|discarded
lazydev after [<project>] <prd> [<prd>...]        # override plan: start only after these are merged
lazydev dev [<project>] <prd> [start|stop]        # dev server in the worktree
lazydev prompt create [--auto] [<description>]    # print the create prompt of the project in the current folder
lazydev prompt unblock [--auto] [<project>] <prd> # print the unblock prompt of a blocked run
lazydev skill install                             # install the Claude Code skill
lazydev mod install|uninstall                     # the Claude Code pane with queue and steps of the runs
lazydev update                                    # install the newest release
lazydev activity [--since 7d] [--json]            # usage report of the TUI
lazydev version                                   # also --version, -v
lazydev                                           # start the TUI
```

- `<project>` is the ID (folder name) or path of a project in the list, `<prd>` the file name without `.md`.
  Without `<project>` the project of the current folder applies, also inside the worktree of a run.
- `--auto` makes Claude answer the questions of create or unblock itself and record them under
  `## Entscheidungen`.
- `wait` is made for an operator session: one line per event (`blocked`, `error`, `aborted`, `merged`,
  `note`, `resolved`), `idle` when nothing runs or waits, `--json` with the details of `status --json`.

## TUI

`lazydev` without arguments opens a terminal UI: projects left, terminal tabs center (Claude Code starts in
each project), details pane below with the runs of the project, PRDs and files right. Useful to watch
several projects and to keep shells, logs and Claude sessions in one place. Everything it does on runs the
CLI does too.

| Key | Action |
|---|---|
| `alt+←/→`, `tab` / `alt+shift+←/→` | pane / tab within the pane |
| `alt+j/k`, `alt+1…9` | project, center tab |
| `alt+t` / `alt+w` | new shell tab / close tab |
| `n` | new PRD with Claude Code (PRDs) |
| `s` / `x` / `X` | start or continue / stop / discard run (PRDs, Runs) |
| `f` | resolve a blocked run with Claude Code in its worktree |
| `L` / `l` | log of the run as a tab (PRDs / Runs), `z` folds its header |
| `D` | dev server of the PRD's worktree on/off |
| `c` | status: planned, merged, discarded |
| `a` / `d` / `i` | add / remove project / `.lazydev/` wizard (Projects) |
| `alt+g` | git cockpit: fetch, pull (`--ff-only`), push, commit (`add -A`), each as a tab |
| `alt+v` | Markdown viewer of the selected file or PRD, `e` opens the editor |
| `alt+c` | copy mode: `j/k`, `v` select lines, `y` copy |
| `alt+u` | install the newer release shown in the footer |
| `alt+m` / `alt+q` | menu (theme, Discord, auto-unblock) / quit |

- `alt` works everywhere, inside a terminal too. Outside a terminal it is optional.
- Copying (copy mode, mouse drag) takes the tab content only and joins wrapped lines. It goes through OSC 52,
  so it works over SSH. Shift+drag gives the native selection.
- Pull and push of the git cockpit fail during a running merge, try again after it.
- Phases: `○` planned, `◌` waiting, `●` working, `◐` merge-queue, `✓` merged, `■` aborted, `✗`
  error/discarded, `⊘` blocked.
- `lazydev activity` reports how you use the TUI (time per pane, keys, reaction times after a run blocked or
  merged). `"activity": false` in `settings.json` switches the log off.

## Files

| Path | Content |
|---|---|
| `~/.config/lazydev/` | `projects.json`, `settings.json` (`theme`, `allowedTools`, `permissionMode`, `discord`, `skill`, `autoUnblock`, `activity`), rights `0600` |
| `~/.local/state/lazydev/` | `runs/<id>/<prd>.json` and step logs, `trust/`, `locks/`, `usage.json`, `activity.jsonl`, `bin/` |
| `~/.local/share/lazydev/worktrees/<id>/<prd>/` | Worktrees |
| `~/.claude/skills/lazydev/`, `lazydev-runs/` | Claude Code skill and mod |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` and `XDG_DATA_HOME` apply, `CLAUDE_CONFIG_DIR` replaces `~/.claude`.

## Development and release

```sh
make test   # go vet ./... and go test ./...
make build  # bin/lazydev
git tag v0.2.0 && git push origin v0.2.0
```

A tag `vX.Y.Z` starts a GitHub Action: `make test`, GoReleaser builds `linux/amd64` and `linux/arm64`,
release with archives and `checksums.txt`. Release notes come from the commits since the previous tag
(Features, Fixes, Performance, Other; `docs`, `test`, `chore`, `ci`, `style`, `build` drop out).
