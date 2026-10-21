# lazydev

<p align="center">
  <b>English</b> · <a href="README.de.md">Deutsch</a>
</p>

Terminal UI for the PRD workflow with Claude Code. Projects on the left, terminal tabs in the center,
files and PRDs on the right. You write a PRD with Claude in a tab, start it with one key, and follow its
log. Each PRD runs in its own Git worktree with its own port, and lazydev fast-forwards it into the
project's base branch. [SPEC.md](SPEC.md) describes the behavior in full.

## Requirements

- Linux (`/proc`, `setsid`, `flock`), Go 1.27+ to build.
- `git` and [Claude Code](https://claude.com/claude-code) (`claude`) in `PATH`.
- Optional: `docker` for the Docker tab, `$VISUAL`/`$EDITOR` for opening files.

## Install

```sh
make install          # ~/.local/bin/lazydev, PREFIX overridable
go run ./cmd/lazydev  # without installing
```

Runners keep the code they started with. After `make install`, restart lazydev.

## Get started

1. `lazydev`, then `a` in the left pane and a project path (`Tab` completes). lazydev creates
   `.lazydev/` with the prompts of the simple workflow. Commit it.
2. Right pane, tab `PRDs`: `n` and a sentence about the feature. Claude Code opens as a center tab and
   writes `prd/<area>/<name>.md` with you.
3. Select the PRD, `s`. Setup, implement, review and merge run in the background.
4. The details pane below the terminals lists the runs of the project with phase, step, cost and the last
   log line. `l` on a run opens its complete log as a center tab, `z` there folds the header.

## Layout

| Pane | Content |
|---|---|
| Projects (left) | Projects with branch and commits ahead/behind upstream |
| Center | Terminal tabs and log tabs of the project. Claude Code starts in a project without a tab |
| Below center | `Details`: runs of the project, without runs the `.lazydev/` setup. `Docker`: compose services and containers |
| Explorer (right) | `PRDs`: PRDs with phase, done ones folded. `Files`: file tree |

## Keys

`alt` works everywhere, inside a terminal too. Outside a terminal, `alt` is optional.

| Key | Action |
|---|---|
| `alt+←/→`, `tab` | pane |
| `alt+shift+←/→` | tab within the pane |
| `alt+j/k`, `alt+1…9` | project, center tab |
| `alt+t` / `alt+w` | new shell tab / close tab |
| `alt+g` | git cockpit: fetch, pull, push, commit |
| `alt+m` / `alt+q` | menu (theme, Discord, git) / quit |
| `j/k`, `l`/`enter`, `h` | move, open, back (lists, tree) |
| `a` / `d` / `i` | add / remove project / `.lazydev/` wizard (Projects) |
| `n` | new PRD with Claude Code (PRDs) |
| `s` / `x` / `X` | start or continue / stop / discard run (PRDs, Runs) |
| `D` | dev server of the PRD's worktree on/off (PRDs, Runs) |
| `L` / `l` | log of the run as a tab (PRDs / Runs) |
| `c` | status: planned, merged, discarded. Commits tracked PRDs (PRDs) |
| `z` | zoom the details pane, header in the log tab |
| `l` | logs of the container as a tab (Docker) |

## `.lazydev/`

Versioned in the project, the same for everyone. Adding a project and the wizard create missing files.

| File | Content |
|---|---|
| `config.yaml` | `baseBranch`, `knowledge`, `allowedTools`, `hooks` |
| `create.md`, `implement.md`, `review.md`, `merge.md` | Prompts of the steps. The workflow reads them from here only |

```yaml
baseBranch: develop          # default main
knowledge:                   # files Claude reads first: all, create, implement, review, merge
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

- Each prompt = header + knowledge block + `.lazydev/<step>.md` + built-in contract. The contract cannot be
  changed per project.
- Hooks get `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`, `LAZYDEV_SLUG`. The
  dev server also gets `PORT`.
- Hooks and `allowedTools` need trust. `s` asks, or `lazydev trust <project>`. Changing them asks again.

## Runs

- Steps: setup (worktree on `feat/<prd>`, untracked PRD moved and committed, setup hook) → implement →
  review → merge (rebase, Claude on conflicts, check hook, cost into the status line, fast-forward,
  teardown hook, cleanup).
- Max 3 runners at a time, the rest waits. Ports from 3500.
- After an error or `x`, `s` continues with the failed step. `X` throws away worktree, branch and its
  commits, the PRD goes back with status `geplant`.
- The merge waits while the main folder is not on the base branch or has changes outside `prd/` and
  `.lazydev/`.
- PRD status values stay German (`geplant`, `umgesetzt (date)`, `verworfen (date)`), like the prompts.

## Git cockpit

`alt+g` shows branch, upstream, changes and the last 5 commits of the selected project's main folder. Each
action runs as a center tab, so passphrase and credential prompts work like in a shell.

| Key | Action | Git |
|---|---|---|
| `f` | Fetch | `git fetch` |
| `p` | Pull | `git pull --ff-only` |
| `P` | Push | `git push`, without upstream `git push -u origin <branch>` |
| `c` | Commit | `git add -A` and `git commit -m <message>` |

- On success the tab closes and the footer shows `git <action>: done`. On failure the tab stays open until
  you press `enter`.
- Pull and push do not wait for a running merge of a PRD. They end with an error, try again once the merge
  is done.
- Commit takes all changes, untracked files, `.lazydev/` and `prd/` included.

## Discord

Runners post to a Discord webhook when a PRD is merged, a step fails, `x` aborts a runner or the merge
starts to wait for the main folder. They post without a running TUI too.

1. In Discord, create a webhook for a channel and copy its URL.
2. `alt+m`, tab `Discord`, `enter`. Select `Global URL`, press `enter`, paste the URL and press `enter`.
3. Switch on `Notifications`. `Send test` posts a test message right away.

A project can mute itself or use its own URL. The URL lands in `~/.config/lazydev/settings.json` only, never
in a repository. The menu shows it shortened to `…/<last 4 characters>`.

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

Each message is an embed with project, branch, cost, duration and model, without error text. A failed
delivery retries 3 times 5 s apart, then the step log gets `Discord notification failed: <error>`. The run
goes on unchanged.

## CLI

```sh
lazydev start <project> <prd>            # start or continue in the background
lazydev stop <project> <prd>
lazydev discard [--yes] <project> <prd>
lazydev trust <project>
```

`<project>` is the ID (folder name) or path of a project in the list, `<prd>` the file name without `.md`.

## Files

| Path | Content |
|---|---|
| `~/.config/lazydev/` | `projects.json`, `settings.json` (`theme`, `allowedTools`, `permissionMode`, `discord`), rights `0600` |
| `~/.local/state/lazydev/` | `runs/<id>/<prd>.json` and step logs, `trust/`, `locks/`, `usage.json`, `bin/` |
| `~/.local/share/lazydev/worktrees/<id>/<prd>/` | Worktrees |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` and `XDG_DATA_HOME` apply.

`allowedTools` in `settings.json` apply to every Claude step of all projects, after the built-in tools and
before `allowedTools` from `config.yaml`. They need no trust. `permissionMode` is `acceptEdits` (default) or
`auto`. Any other value fails the Claude step before Claude starts.

```json
{
  "permissionMode": "auto",
  "allowedTools": ["Bash(head:*)", "Bash(tail:*)", "Bash(sed -n:*)", "Bash(grep:*)"]
}
```

In mode `auto` a classifier of Claude Code decides on each tool call instead of the prefix list. The outcome
is not deterministic.

## Development

```sh
make test   # go vet ./... and go test ./...
make build  # bin/lazydev
```
