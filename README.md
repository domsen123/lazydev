# lazydev

<p align="center">
  <b>English</b> · <a href="README.de.md">Deutsch</a>
</p>

lazydev is a terminal UI for the PRD workflow with Claude Code. You see all projects and their PRDs, start
the implementation with a single key, and follow the log live. Each PRD runs in its own Git worktree with
its own database and port. After the review, lazydev fast-forward merges the branch into `main` and cleans
up.

The UI, the prompts, and [SPEC.md](SPEC.md), the full description of the behavior, are in German. This
README gives the German UI labels in quotes, followed by a translation.

## Requirements

- Linux with `/proc`. lazydev uses `setsid`, `flock`, and `/proc` for processes and ports.
- Go 1.27 or later to build.
- `git` and [Claude Code](https://claude.com/claude-code) (`claude`) in your `PATH`.
- The global commands `/create-prd`, `/implement-prd`, `/arewedone`, and `/merge-prd` in
  `~/.claude/commands/`, and the `committer` agent.
- For the `nuxt-drizzle` preset: Docker, `pnpm`, and the project's Compose containers.
- For the "Issues" box: the [GitHub CLI](https://cli.github.com) (`gh`) in your `PATH`, signed in with
  `gh auth login`.
- Optional: [Herdr](https://herdr.dev). Inside Herdr, `o`, `w`, and `L` open a new pane instead of
  suspending the UI.

## Installation

```sh
make install
```

This command installs the binary to `~/.local/bin/lazydev`. To use a different location, set `PREFIX`, for
example `make install PREFIX=/usr/local`.

A running TUI keeps its code. To use a new version, run `make install` and restart lazydev. Runners that
start afterwards use the new version. For development, `go run ./cmd/lazydev` starts the TUI without
installing it.

## Get started

1. Start `lazydev`.
2. Press `a`, enter the path of a project, and confirm with `Enter`. `Tab` completes the path. lazydev
   detects the `nuxt-drizzle` preset from `drizzle.config.ts` and `server/database/migrations`.
3. Press `n`, choose a template, and describe the feature in one sentence. Claude asks its questions in the
   chat window. The finished PRD appears untracked at `prd/<area>/<name>.md`.
   Each round keeps running in the background, even after `Esc` or `q`. When Claude asks a question or a
   round fails, the chat appears in the "Benachrichtigungen" (notifications) box to the right of the PRDs.
   The project then shows ` <count>✉`. To reopen the chat, press `N`, or `Enter` in the box.
   - **Nuxt-Feature** uses `/create-prd` and `/implement-prd`, with a data model and a UI handoff.
   - **Einfach** (simple) is for projects without a data model, UI handoff, or architecture docs. lazydev
     ships its own prompts for it. To allow check commands such as `make test`, add them to the
     project's `allowedTools`, for example `Bash(make test:*)`.
4. Select the PRD and press `s`. lazydev creates the worktree, sets up the databases, runs `/implement-prd`
   (its own prompt for "Einfach") and `/arewedone`, and merges.
5. Follow the run in the log at the bottom. Press `l` to show it at full height.

The bottom of the PRD list shows the project's Docker services with status, ports, and image, followed by
the running dev servers. To start or stop containers or to see their logs, press `D`.

If a project's `origin` points to `github.com`, lazydev loads up to 30 of the newest open issues through
`gh`. It does so at startup, every 30 minutes, and when you press `r`. If the project has open issues, the
"Issues" box appears in the right column, below "Benachrichtigungen" or on its own. The project shows
` <count>#`. In the box, `Enter` shows the issue in a window and `o` opens it in the browser. To turn an
issue into a PRD, press `n`, choose the template, and confirm with `Enter`. The chat starts with the
issue's title and body. After a restart, lazydev shows the last state right away. If `gh` fails, the status
line shows the reason.

## Keys

The footer shows only the keys that work in the focused box and for the selected PRD. All other keys still
work, and `?` shows the full list.

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move the selection in the focused list |
| `Tab` / `←` `→` | Move the focus: projects, PRDs, notifications, issues, log, or details. In the bottom area, `j` `k` scroll. |
| `a` / `d` | Add a project / remove it from the list |
| `n` | New PRD: choose a template (Nuxt-Feature or Einfach), then a chat window with Opus |
| `N` | Open the oldest open question of a chat across all projects |
| `Enter` / `X` | In "Benachrichtigungen": open the chat / end the chat, with confirmation |
| `Enter` / `o` / `n` | In "Issues": show the issue in a window / open it in the browser / start a PRD chat from the issue |
| `Esc` | In the chat: hide the window. The round keeps running. |
| `Ctrl+X` | In the chat: end the chat, with confirmation. A running round stops, the log stays. |
| `Ctrl+R` | In the chat: retry the last round after an error |
| `v` | Show the full PRD as Markdown in a window. Inside, `o` opens the file in your editor. |
| `s` | Start the implementation. After an error or an abort, it continues at the failed step. |
| `m` | Approve the merge when `autoMerge` is off or the merge failed |
| `x` | Stop the runner. The worktree stays. |
| `X` | Discard the PRD: delete databases, worktree, and branch, and move the PRD back to the main folder |
| `o` | Open the PRD's last Claude session in the worktree |
| `p` | Start or stop the dev server in the worktree |
| `w` | Open a shell in the worktree |
| `e` | Expand or collapse finished PRDs. They start collapsed. |
| `S` | Sort PRDs by name or by creation time, newest first |
| `D` | Docker view: `U` compose up, `S` compose stop, `L` logs |
| `l` | Log at full height |
| `Enter` | In the log: show the full event |
| `u` | Claude limits and usage. `r` refreshes them with a short request to Haiku. |
| `r` | Reload projects and issues |
| `?` | Help |
| `q` | Quit. Running runners and chats keep working. |

## Commands without the TUI

```sh
lazydev start <project-id> <prd-name>             # runner in the background, respects maxParallel
lazydev run <project-id> <prd-name>               # runner in the foreground
lazydev merge <project-id> <prd-name>             # approve the merge when autoMerge is off
lazydev stop <project-id> <prd-name>              # stop the runner
lazydev teardown [--yes] <project-id> <prd-name>  # discard the run
lazydev chat <project-id> <chat-id>               # one round of a PRD chat, the TUI starts it
```

The project ID is the name of the project folder. The PRD name is the file name without `.md`. The chat ID
is the chat's start time, for example `2026-10-20-140211`.

## Configuration

lazydev reads `~/.config/lazydev/config.yaml`. To use a different file, set `LAZYDEV_CONFIG`. The TUI
writes the file itself when you add or remove projects.

```yaml
maxParallel: 3               # concurrent runners across all projects
portBase: 3100               # first port for worktrees
claude:
  binary: claude
projects:
  - path: ~/development/nuxt/nuxt-drizzle
    preset: nuxt-drizzle
    autoMerge: true          # default. false: merge only after m
    allowedTools:            # extends the tools Claude may use
      - "Bash(pnpm test:e2e:*)"
    hooks:                   # replaces single hooks of the preset
      setup: ./scripts/lazydev-setup.sh
      teardown: ./scripts/lazydev-teardown.sh
      check: ./scripts/lazydev-check.sh
      dev: ./scripts/lazydev-dev.sh
```

Hook scripts run in the worktree. Relative paths start at the project's main folder. The scripts receive
`LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`, `LAZYDEV_BRANCH`, `LAZYDEV_PORT`, and `LAZYDEV_SLUG`.

## The nuxt-drizzle preset

- **Setup:**
  - copies `.env` and `.env.test` into the worktree,
  - appends `_<slug>` to `NUXT_DATABASE_NAME` and sets `PORT`,
  - creates both databases with `docker exec … createdb`,
  - runs `pnpm install`, `pnpm db:migrate`, and `pnpm test:db:push`.
- **Checks before the merge:** `pnpm lint`, `pnpm typecheck`, `pnpm test:run`.
- **Dev server:** `pnpm dev` on the worktree's port.
- **Teardown:** deletes only databases with `_<slug>` in their name. The main databases stay untouched.

## Files

| What | Path |
|---|---|
| Configuration | `~/.config/lazydev/config.yaml` |
| Run state | `~/.local/state/lazydev/runs/<project-id>/<prd-name>.json` |
| Log per step | `~/.local/state/lazydev/runs/<project-id>/<prd-name>.<step>.jsonl` |
| Runner output | `~/.local/state/lazydev/runs/<project-id>/<prd-name>.runner.log` |
| Dev server | `~/.local/state/lazydev/runs/<project-id>/<prd-name>.dev.log` |
| Last state of the Claude limits | `~/.local/state/lazydev/usage.json` |
| State of the chat for `n` | `~/.local/state/lazydev/chats/<project-id>/<chat-id>.json` |
| Log of the chat for `n` | `~/.local/state/lazydev/chats/<project-id>/<chat-id>.jsonl` |
| Last state of the GitHub issues | `~/.local/state/lazydev/issues/<project-id>.json` |
| Worktrees | `~/.local/share/lazydev/worktrees/<project-id>/<prd-name>/` |

lazydev respects `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and `XDG_DATA_HOME`.

## Troubleshooting

- **A PRD shows "Fehler" (error).** The header line of the log names the reason. Press `o` to ask in the
  session, or `s` to restart the step.
- **A PRD shows "abgebrochen" (aborted) although you didn't press `x`.** The runner ended unexpectedly. Its
  output is in `<prd-name>.runner.log`.
- **The merge waits with "Hauptordner nicht sauber" (main folder not clean).** Commit or discard the changes
  in the main folder. The merge checks again every 5 seconds.
- **The chat for `n` shows a red error such as `Brace expansion`.** Claude tried a command that the chat
  doesn't allow and then reads the files with Read and Grep. The PRD is still created. The chat log
  contains all events.
- **Claude denies a command.** The line `abgelehnt: …` (denied) in the log names the tool. To allow it, add
  it to `allowedTools` in the project configuration.

## Development

```sh
make test     # go vet and go test
make build    # binary at bin/lazydev
```
