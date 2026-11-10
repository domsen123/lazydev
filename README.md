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

`install.sh` downloads a release from GitHub. It needs `curl`, `tar`, `sha256sum` and `uname`.

```sh
curl -fsSL https://raw.githubusercontent.com/domsen123/lazydev/main/install.sh | sh
sh install.sh v0.2.0  # a specific release instead of the newest
```

The script checks the checksum and installs `lazydev` to `$PREFIX/bin`, `PREFIX` defaults to `~/.local`.
`lazydev version` prints the installed version. On start the TUI checks for a newer release and shows it in
yellow in the footer and in the menu tab `General`. `alt+u` installs it over the running binary, `lazydev
update` does the same from the shell. Running `install.sh` again works too.

From source, with Go 1.27+:

```sh
make install          # ~/.local/bin/lazydev, PREFIX overridable
go run ./cmd/lazydev  # without installing
```

Runners keep the code they started with. After an update, restart lazydev.

## Claude Code skill

lazydev installs a skill `lazydev` for Claude Code into `~/.claude/skills/lazydev/`. With it, a normal
Claude Code session outside lazydev knows what lazydev is, how PRDs and `.lazydev/` work, and how to write
a PRD. Discuss a feature in any Claude Code session in a lazydev project and say "make this a lazydev
PRD": Claude runs `lazydev prompt create` in the main folder, gets the same prompt `n` builds, asks its
numbered questions with options A to D and writes `prd/<area>/<name>.md`. Questions about lazydev go to
the specification the skill carries along.

The skill knows the CLI, so Claude never sends you to the TUI to press a key: it starts and stops runs,
watches them with `lazydev wait`, reads `lazydev status` and resolves blockers with
`lazydev prompt unblock`. Say "answer the questions yourself" and it adds `--auto`: Claude answers the
questions of create or unblock from the conversation and the code and records them under
`## Entscheidungen` in the PRD.

The TUI writes the skill at its start when it is missing or out of date, so it follows every update.
`lazydev add` and `lazydev skill install` do the same from the shell. `"skill": false` in `settings.json` switches the
automatic write off. To remove the skill, delete the folder and set `"skill": false`.

## Claude Code mod

`lazydev mod install` adds a pane to Claude Code that shows the runs of the project beside the chat, like
the agents panel: cost, running and waiting runs at the top, then each running PRD with phase, model, port,
`3/5 · implement` with a progress bar, cost and time, the blocked runs with their questions, the queue
with what each PRD waits for, and the planned PRDs. It opens by itself once a run works, waits or is
blocked; `/lazydev-runs` shows or hides it. It reads `lazydev status --here --json` every 3 s, so it shows only the project of the session's folder.

The mod lives in `~/.claude/skills/lazydev-runs/`, the TUI keeps it current after updates, and
`lazydev mod uninstall` removes it. New Claude Code sessions load it, `/reload-plugins` a running one.

## Release

Push a tag `vX.Y.Z`. A GitHub Action runs `make test`, builds `linux/amd64` and `linux/arm64` with
GoReleaser and creates the release with the archives and `checksums.txt`. The release notes come from the
commits since the previous tag, grouped into Features, Fixes, Performance and Other. Commits of type `docs`,
`test`, `chore`, `ci`, `style` and `build` do not show.

```sh
git tag v0.2.0
git push origin v0.2.0
```

## Get started

1. `lazydev`, then `a` in the left pane and a project path (`Tab` completes). lazydev creates
   `.lazydev/` with the prompts of the simple workflow. Commit it.
2. Right pane, tab `PRDs`: `n` and a sentence about the feature. Claude Code opens as a center tab and
   writes `prd/<area>/<name>.md` with you.
3. Select the PRD, `s`. Plan, setup, implement, review and merge run in the background.
4. The details pane below the terminals lists the runs of the project with phase, step, cost and the last
   log line. `l` on a run opens its complete log as a center tab, `z` there folds the header.

## Layout

| Pane | Content |
|---|---|
| Projects (left) | Projects with branch and commits ahead/behind upstream |
| Center | Terminal tabs, log tabs and Markdown viewers of the project. Claude Code starts in a project without a tab |
| Below center | `Details`: runs of the project, without runs the `.lazydev/` setup. `Docker`: compose services and containers |
| Explorer (right) | `PRDs`: PRDs with phase, newest first, done ones folded. `Files`: file tree |

## Keys

`alt` works everywhere, inside a terminal too. Outside a terminal, `alt` is optional.

| Key | Action |
|---|---|
| `alt+←/→`, `tab` | pane |
| `alt+shift+←/→` | tab within the pane |
| `alt+j/k`, `alt+1…9` | project, center tab |
| `alt+t` / `alt+w` | new shell tab / close tab |
| `alt+g` | git cockpit: fetch, pull, push, commit |
| `alt+c` | copy mode of the terminal tab |
| `alt+v` | Markdown viewer of the selected file or PRD |
| `alt+u` | install the newer release, if one is shown |
| `alt+m` / `alt+q` | menu (theme, Discord, git) / quit |
| `j/k`, `l`/`enter`, `h` | move, open, back (lists, tree) |
| `a` / `d` / `i` | add / remove project / `.lazydev/` wizard (Projects) |
| `n` | new PRD with Claude Code (PRDs) |
| `s` / `x` / `X` | start or continue / stop / discard run (PRDs, Runs) |
| `D` | dev server of the PRD's worktree on/off (PRDs, Runs) |
| `f` | resolve a blocked run with Claude Code in its worktree (PRDs, Runs) |
| `L` / `l` | log of the run as a tab (PRDs / Runs) |
| `c` | status: planned, merged, discarded. Commits tracked PRDs (PRDs) |
| `z` | zoom the details pane, header in the log tab |
| `l` | logs of the container as a tab (Docker) |

## Copying from terminal tabs

Copying takes the content of the tab only, without projects, explorer and borders. Lines that wrapped at
the tab width come out as one line.

- **Copy mode:** `alt+c` freezes the tab with its scrollback. `j/k`, `ctrl+d/u` and `g/G` move the mark, `v`
  starts a selection of whole lines, `y` copies and leaves, `esc` or `q` leave. The tab label ends with
  ` [copy]`. The program keeps running, its output shows once you leave.
- **Drag:** drag with the left button in the tab to select exact to the character. Releasing copies.
- **Wheel:** scrolls through the scrollback. Wheel down to the end shows the running output again.
- **Native selection:** hold Shift while dragging, like in most terminal programs.

lazydev copies through OSC 52 to its terminal, so it works over SSH too, and outside SSH also into the local
clipboard. Some terminals turn OSC 52 off or limit its size.

## Reading Markdown

`alt+v` (or `v` outside a terminal) opens the Markdown file marked in the `Files` tab, or the PRD marked in the
`PRDs` tab, rendered in a center tab `md <file>`. A PRD in the worktree of a run shows from there, and its
viewer closes with the run. `j/k`, `ctrl+d/u` and `g/G` scroll, so does the mouse wheel. `e` opens the file in
the editor as a new tab, the viewer stays. The viewer renders again within about a second when the file
changes, and closes with a note when the file is gone. `alt+w` closes it.

## `.lazydev/`

Versioned in the project, the same for everyone. Adding a project and the wizard create missing files.

| File | Content |
|---|---|
| `config.yaml` | `baseBranch`, `knowledge`, `allowedTools`, `hooks`, `databases` |
| `create.md`, `implement.md`, `review.md`, `merge.md`, `unblock.md` | Prompts of the steps. The workflow reads them from here only |

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

- Each prompt = header + knowledge block + `.lazydev/<step>.md` + built-in contract. The contract cannot be
  changed per project.
- Hooks get `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`, `LAZYDEV_SLUG`. The
  dev server also gets `PORT`.
- Hooks, `allowedTools` and `databases` need trust. `s` asks, or `lazydev trust <project>`. Changing them
  asks again.

### Databases per worktree

For projects with Postgres in Docker Compose, `databases` gives every worktree its own empty database
`<name>_<slug>`. The wizard (`i`) finds Postgres services in the compose file and writes the section.

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

- With `DATABASE_URL="postgres://app:secret@localhost:5432/app?sslmode=disable"` in the main folder and the
  slug `login_form`, the worktree gets `DATABASE_URL="postgres://app:secret@localhost:5432/app_login_form?sslmode=disable"`.
  In a URL only the path changes, a plain value like `app_test` becomes `app_test_login_form`.
- The setup copies the env files into the worktree, starts a stopped service with `docker compose up -d`,
  creates the empty databases and then runs the setup hook. An existing database stays.
- The user is `POSTGRES_USER` of the service, `postgres` without it. lazydev runs everything through
  `docker exec`.
- If the base branch moved before the merge, lazydev creates the databases again after the rebase and runs
  the setup hook again before the check hook.
- The merge and `X` drop the databases after the teardown hook. lazydev only drops names ending in
  `_<slug>`, the main databases stay.

Migrations and seeds stay in the setup hook, e.g.:

```sh
#!/bin/sh
# scripts/lazydev-setup.sh: the worktree's .env already names its own database.
set -e
pnpm install --frozen-lockfile --prefer-offline
pnpm db:migrate
```

## Runs

- Steps: plan (only with other open runs in the project: Claude reads the PRDs and the code and decides
  whether the PRD runs in parallel or waits until some of them are merged) → setup (worktree on `feat/<prd>`, untracked PRD moved and committed, databases, setup hook) →
  implement → review → merge (rebase, Claude on conflicts, databases again if the base moved, check hook,
  cost into the status line, fast-forward, teardown hook, dropping the databases, cleanup).
- Max 3 runners at a time, the rest waits. A PRD that plan holds back waits without taking a slot and starts
  once the PRDs it waits for are merged or discarded. `lazydev after <prd> [<prd>…]` overrides the order,
  `lazydev status` and the run list show what a PRD waits for and why. Ports from 3500.
- After an error or `x`, `s` continues with the failed step. `X` throws away worktree, branch and its
  commits, the PRD goes back with status `geplant` and without `## Blocker`.
- The merge waits while the main folder is not on the base branch or has changes outside `prd/` and
  `.lazydev/`. If the base branch moved meanwhile, it rebases again.
- PRD status values stay German (`geplant`, `umgesetzt (date)`, `verworfen (date)`, `blockiert (date)`), like
  the prompts.
- Phases: `○` planned, `◌` waiting, `●` plan/setup/implement/resolve/review/merge, `◐` merge-queue, `✓` merged, `■`
  aborted, `✗` error/discarded, `⊘` blocked (yellow).

### Blocked runs

When the PRD leaves a decision open, Claude aborts implement or the Claude step merge with open questions.
That is no error: the run ends in phase `blocked`.

1. The runner sets the PRD in the worktree to `blockiert (date)`, appends `## Blocker` with the step and all
   questions and commits only the PRD. Discord gets `⊘ Blocked`.
2. `f` on the run opens Claude Code in the worktree as tab `unblock`. It asks the questions with options A
   to D, works your answers into the PRD, removes `## Blocker` and sets the status back (`geplant` for
   implement, `umgesetzt (date)` for merge). It changes only the PRD and commits nothing.
3. `s` continues with the blocked step. While the PRD still says `blockiert`, `s` refuses.

Without the TUI, `lazydev prompt unblock [--auto] <project> <prd>` prints the same prompt for any Claude
Code session, and `lazydev start` continues.

**Auto-unblock** (opt-in per project: `lazydev auto-unblock on` or `alt+m` → General): when a run gets
blocked, the runner lets Claude answer the questions itself in the worktree. It records each answer with a
reason under `## Entscheidungen`, commits the PRD alone and runs the blocked step again; Discord gets
`↻ Unblocked by Claude` with the decisions. Questions about data loss, security, public interfaces or costs
it may not decide, then the run stops as before. At most 2 times per run.

Remarks without a decision go under `## Hinweise` in the report of implement and do not block.

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

Runners post to a Discord webhook when a PRD is merged, a step fails or blocks, Claude resolves a blocker
(auto-unblock), `x` aborts a runner or the merge starts to wait for the main folder. They post without a running TUI too.

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

Each message is an embed with project, branch, cost, duration and model, without error text. A blocked run
(`⊘ Blocked`) adds the field `Blocker` with its open questions. A failed
delivery retries 3 times 5 s apart, then the step log gets `Discord notification failed: <error>`. The run
goes on unchanged.

## CLI

The TUI is optional, every action has a command:

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
lazydev skill install                             # install the Claude Code skill into ~/.claude/skills/lazydev/
lazydev mod install|uninstall                     # the Claude Code pane with queue and steps of the runs
lazydev activity [--since 7d] [--json]            # what you did in the TUI: time per pane, keys, runs, reaction times
lazydev version                                   # also --version, -v
```

`<project>` is the ID (folder name) or path of a project in the list, `<prd>` the file name without `.md`.
Without `<project>`, the project of the current folder applies, also from inside the worktree of a run.
`prompt create` works in any folder of a project with `.lazydev/`, the project needs no entry in the list.
`--auto` makes Claude answer the questions itself and record them under `## Entscheidungen`.

`wait` is made for an operator session: it returns with one line per event (`blocked`, `error`,
`aborted`, `merged`, `note`, `resolved`), with `idle` when nothing runs or waits, and `--json` gives the details of
`status --json` along. A running TUI picks up projects that `add` and `remove` change within 5 s.

`activity` reads the log the TUI writes while it runs: every key outside a terminal with its pane, tab and
dialog, the time spent in each terminal tab, actions, and the phase changes of runs. The report shows sessions
and active time, time and keys per pane, the keys you use most, and how long you took to react after a run
blocked, failed or merged. `--json` prints the raw events, e.g. for Claude to read. `"activity": false` in
`settings.json` switches the log off.

## Files

| Path | Content |
|---|---|
| `~/.config/lazydev/` | `projects.json`, `settings.json` (`theme`, `allowedTools`, `permissionMode`, `discord`, `skill`, `activity`), rights `0600` |
| `~/.local/state/lazydev/` | `runs/<id>/<prd>.json` and step logs, `trust/`, `locks/`, `usage.json`, `activity.jsonl`, `bin/` |
| `~/.local/share/lazydev/worktrees/<id>/<prd>/` | Worktrees |
| `~/.claude/skills/lazydev/` | Claude Code skill: `SKILL.md`, `references/spec.md` |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` and `XDG_DATA_HOME` apply, `CLAUDE_CONFIG_DIR` replaces `~/.claude`.

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
