# lazydev

<p align="center">
  <b>English</b> · <a href="README.de.md">Deutsch</a>
</p>

lazydev is a terminal UI for the PRD workflow with Claude Code. You see all projects and their PRDs, start
the implementation with a single key, and follow the log live. Each PRD runs in its own Git worktree with
its own database and port. After the review, lazydev fast-forward merges the branch into the project's
target branch (`main` by default) and cleans up. Before the fast-forward, lazydev records the total cost of
the run and the model of the implementation in the PRD's status line (`Kosten: 4.12 USD · Modell:
claude-opus-5-5`), so every machine and team member sees them in the PRD list.

The UI, the prompts, and [SPEC.md](SPEC.md), the full description of the behavior, are in German. This
README gives the German UI labels in quotes, followed by a translation.

## Requirements

- Linux with `/proc`. lazydev uses `setsid`, `flock`, and `/proc` for processes and ports.
- Go 1.27 or later to build.
- `git` and [Claude Code](https://claude.com/claude-code) (`claude`) in your `PATH`.
- No global commands or agents. lazydev ships default prompts for create, implement, review, and merge, and
  the `committer` and `structural-completeness-reviewer` agents in the binary. It passes the agents with
  `claude --agents`. Each project can replace the prompts in its `.lazydev/` folder.
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
2. Press `a`, enter the path of a project, and confirm with `Enter`. `Tab` completes the path.
3. Press `i` to create the project's `.lazydev/` folder (see below). Commit the folder so your team gets
   the same settings.
4. Press `n` and describe the feature in one sentence. Claude asks its questions in the chat window. The
   finished PRD appears untracked at `prd/<area>/<name>.md`.
   Each round keeps running in the background, even after `Esc` or `q`. When Claude asks a question or a
   round fails, the chat appears in the "Benachrichtigungen" (notifications) box to the right of the PRDs.
   The project then shows ` <count>✉`. To reopen the chat, press `N`, or `Enter` in the box.
5. Select the PRD and press `s`. lazydev creates the worktree, sets up the databases, runs the implement
   and review steps, and merges. If `.lazydev/config.yaml` sets hooks or allowed tools, lazydev first asks
   you to allow them.
6. Follow the run in the log at the bottom. Press `l` to show it at full height.

The bottom of the PRD list shows the project's Docker services with status, ports, and image, followed by
the running dev servers. To start or stop containers or to see their logs, press `D`.

If a project's `origin` points to `github.com`, lazydev loads up to 30 of the newest open issues through
`gh`. It does so at startup, every 30 minutes, and when you press `r`. If the project has open issues, the
"Issues" box appears in the right column, below "Benachrichtigungen" or on its own. The project shows
` <count>#`. In the box, `Enter` shows the issue in a window and `o` opens it in the browser. To turn an
issue into a PRD, press `n`. The chat starts right away with the issue's title and body. After a restart,
lazydev shows the last state right away. If `gh` fails, the status line shows the reason.

## The project folder `.lazydev/`

Each project keeps its lazydev settings in its repository, in the `.lazydev/` folder of the main folder.
Everyone on the team and every worktree uses the same settings. Nothing in it can be overridden locally.

| File | Content |
|---|---|
| `.lazydev/config.yaml` | `baseBranch`, `preset`, `knowledge`, `allowedTools`, `hooks` |
| `.lazydev/create.md` | Prompt for the PRD chat |
| `.lazydev/implement.md` | Prompt for the implementation |
| `.lazydev/review.md` | Prompt for review and commit |
| `.lazydev/merge.md` | Prompt for conflicts and failed checks during the merge |

```yaml
# .lazydev/config.yaml
baseBranch: next          # target branch, default main. Release to main stays manual.
preset: nuxt-drizzle      # setup, teardown, checks, env files. Default: no preset
knowledge:                # files Claude reads first, relative to the project
  all: [CLAUDE.md]        # every step
  implement: [_ARCHITECTURE.md]
allowedTools:             # extends the tools Claude may use in implement, review, and merge
  - "Bash(pnpm test:e2e:*)"
hooks:                    # replaces single hooks of the preset
  setup: ./scripts/lazydev-setup.sh
  teardown: ./scripts/lazydev-teardown.sh
  check: ./scripts/lazydev-check.sh
  dev: ./scripts/lazydev-dev.sh
```

- **Prompts:** If a prompt file is missing, lazydev uses its default prompt, the general "einfach"
  (simple) workflow. A prompt file may start with a frontmatter block between `---` lines, lazydev removes
  it, so you can reuse commands from `.claude/commands/`. Prompts can use `$ARGUMENTS`, `$BASE` (target
  branch), and `$BRANCH`.
- **Contract:** lazydev appends a fixed contract to every prompt, starting with `## Regeln von lazydev`
  (lazydev's rules). It contains what the runner checks, such as the report format and the status line,
  and forbids changes to `.lazydev/`. If Claude changes a file under `.lazydev/` in the worktree, the step
  fails.
- **Knowledge:** Files under `knowledge.all` go into every prompt, followed by those of the step, as a list
  under `## Pflichtlektüre` (required reading). If a file is missing in the step's folder, the step fails
  with `Knowledge-Datei fehlt: <path>` (knowledge file missing) without starting Claude.
- **Trust:** `hooks` and `allowedTools` run code or extend Claude's permissions. lazydev only starts a
  runner after you allow them: `s` asks in the TUI, `lazydev trust <project-id>` asks on the command line.
  Any change to these two sections asks again.
- lazydev reads the files from the main folder when each step or chat round starts. Uncommitted changes
  apply right away.

**Wizard:** `i` creates the folder in four pages: target branch, knowledge files, steps per file, and a
summary. `Enter` goes forward, `Backspace` back, and `Esc` cancels. The wizard writes `config.yaml` and
copies the four default prompts into the folder so you can adapt them. It takes over `baseBranch`,
`preset`, `allowedTools`, and `hooks` from the central configuration and then removes them there. lazydev
doesn't commit the folder.

**Projects without `.lazydev/`** keep reading `baseBranch`, `preset`, `allowedTools`, and `hooks` from the
central configuration. The title of the PRD box shows ` · ohne .lazydev` (without .lazydev).

## Keys

The footer shows only the keys that work in the focused box and for the selected PRD. All other keys still
work, and `?` shows the full list.

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move the selection in the focused list |
| `Tab` / `←` `→` | Move the focus: projects, PRDs, notifications, issues, log, or details. In the bottom area, `j` `k` scroll. |
| `a` / `d` | Add a project / remove it from the list |
| `b` | Choose the project's target branch and write it to `.lazydev/config.yaml`. Worktrees branch off it, and merges go there. |
| `i` | Create the project's `.lazydev/` folder with the wizard |
| `n` | New PRD: describe the feature, then a chat window with Opus |
| `N` | Open the oldest open question of a chat across all projects |
| `Enter` / `X` | In "Benachrichtigungen": open the chat / end the chat, with confirmation |
| `Enter` / `o` / `n` | In "Issues": show the issue in a window / open it in the browser / start a PRD chat from the issue |
| `Esc` | In the chat: hide the window. The round keeps running. |
| `Ctrl+X` | In the chat: end the chat, with confirmation. A running round stops, the log stays. |
| `Ctrl+R` | In the chat: retry the last round after an error |
| `v` | Show the full PRD as Markdown in a window. Inside, `o` opens the file in your editor. |
| `s` | Start the implementation. After an error or an abort, it continues at the failed step. Asks first to allow hooks and allowed tools from `.lazydev/config.yaml`. |
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
lazydev trust <project-id>                        # review and allow hooks and allowed tools from .lazydev/config.yaml
```

The project ID is the name of the project folder. The PRD name is the file name without `.md`. The chat ID
is the chat's start time, for example `2026-10-20-140211`. Without a matching confirmation, `start` and
`run` fail and point to `lazydev trust`.

## Configuration

lazydev reads `~/.config/lazydev/config.yaml`. To use a different file, set `LAZYDEV_CONFIG`. The TUI
writes the file itself when you add or remove projects. Project settings live in each project's
`.lazydev/config.yaml`.

```yaml
maxParallel: 3               # concurrent runners across all projects
portBase: 3100               # first port for worktrees
claude:
  binary: claude
projects:
  - path: ~/development/nuxt/nuxt-drizzle
    autoMerge: true          # default. false: merge only after m. Stays local on purpose.
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

The wizard detects the preset from `drizzle.config.ts` and `server/database/migrations`.

## Files

| What | Path |
|---|---|
| Configuration | `~/.config/lazydev/config.yaml` |
| Project settings and prompts | `<main folder>/.lazydev/` |
| Confirmed hooks and allowed tools | `~/.local/state/lazydev/trust/<project-id>.json` |
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
  in the main folder. Changes under `.lazydev/` don't block the merge.
  The merge checks again every 5 seconds.
- **The chat for `n` shows a red error such as `Brace expansion`.** Claude tried a command that the chat
  doesn't allow and then reads the files with Read and Grep. The PRD is still created. The chat log
  contains all events.
- **Claude denies a command.** The line `abgelehnt: …` (denied) in the log names the tool. To allow it, add
  it to `allowedTools` in `.lazydev/config.yaml`.
- **A step fails with `Knowledge-Datei fehlt`.** The file isn't in the worktree, usually because it isn't
  committed. Commit it or remove it from `knowledge`.

## Development

```sh
make test     # go vet and go test
make build    # binary at bin/lazydev
```
