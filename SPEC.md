# lazydev specification

Binding description of lazydev's behavior. Code, README.md and README.de.md follow it. Change it first.

## Goal

One terminal UI for the PRD workflow with Claude Code: write a PRD with Claude, implement and review it
without a human in an isolated worktree, merge it by fast-forward, and see everything live.

Not goals: remote runs, pushing, pull requests, issue trackers, editing PRDs inside lazydev.

## Projects

- The project list lives in `$XDG_CONFIG_HOME/lazydev/projects.json` (`{"projects": [<absolute path>]}`).
  `a` adds a folder (`~` expands, `Tab` completes folders), `d` removes it after `y`. Removing closes the
  project's terminals and keeps the folder.
- Adding a project, also one already in the list, runs `.lazydev/` init (see below).
- The project ID is the folder name. Projects that share a folder name get `-<first 6 hex of sha1(path)>`.
  IDs name state and worktree folders and are independent of the list order.
- `settings.json` holds the color theme by name (`Default`, `Tokyo Night`).

## `.lazydev/`

Folder in the main folder of a project, versioned with it. Nothing can be overridden locally.

| File | Content |
|---|---|
| `config.yaml` | `baseBranch` (default `main`), `knowledge`, `allowedTools`, `hooks` (`setup`, `teardown`, `check`, `dev`) |
| `<step>.md` | Prompt of `create`, `implement`, `review`, `merge` |

- Unknown fields in `config.yaml` are ignored, e.g. `preset` of older versions. Broken YAML is an error shown in
  the details pane, runs refuse to start.
- **Init** creates missing files only: `config.yaml` (empty, or `baseBranch` when the repo has neither `main`
  nor `master` but a checked out branch) and the four prompts of the built-in simple workflow. It never
  overwrites. Adding a project, opening the wizard, `n`, and writing a prompt in the wizard run it, so a
  deleted `.lazydev/` comes back.
- The workflow reads prompts from `.lazydev/<step>.md` only. A missing prompt fails the step with
  `.lazydev/<step>.md is missing, press i to add it`.
- lazydev commits nothing under `.lazydev/`.

### Prompt

A prompt consists of, separated by blank lines:

1. Header `# lazydev <step>: <argument>`. Without argument `<branch> → <base>`. Logs show it.
2. Knowledge block `## Pflichtlektüre` listing the files of `knowledge.all` and `knowledge.<step>`, without
   duplicates. Each file must exist where Claude runs (main folder for create, worktree otherwise).
3. `.lazydev/<step>.md` without frontmatter, with `$ARGUMENTS`, `$BASE`, `$BRANCH` replaced.
4. The built-in contract of the step (`internal/prompts/contract/`). It has priority, a project cannot change
   it.

Arguments: create gets the user's description, implement the PRD path, review none, merge the branch plus an
optional error context. Prompts, contracts and PRD status values are German.

### Trust

`hooks` and `allowedTools` run code or widen Claude's rights. Their SHA-256 over the YAML of just these two
sections is trusted per project ID in `$XDG_STATE_HOME/lazydev/trust/<id>.json`. Comments and other fields do
not change it. Without both sections there is nothing to trust. `s` on an untrusted project asks
`.lazydev/config.yaml sets <hooks, n allowed tools>. Trust it and start <prd>? (y/n)`. `lazydev trust`
prints both sections and asks. Runners check trust before every start and again before every hook, so a
hook changed during a run does not run. `D` needs trust too. `X` on an untrusted project cleans up without
the teardown hook and reports `teardown hook not run`.

### Wizard (`i`)

Pages: base branch (local branches without `feat/`) → knowledge files (tracked `.md` outside `prd/` and
`.lazydev/`, `space` marks) → steps per file (`1`–`4` toggle create, implement, review, merge, new files get
create and implement) → prompts (`o` opens one in the editor as a center tab and hides the wizard until the
editor ends, `r` resets it to the simple workflow, an existing one after `y`) → summary with the resulting `config.yaml`, `enter`
saves.

- Without knowledge in `config.yaml`, `CLAUDE.md`, `ARCHITECTURE.md` and `_ARCHITECTURE.md` are preselected.
- A file marked for all four steps goes under `all`.
- Saving writes `baseBranch` (left out for `main`) and `knowledge` only and keeps other fields, order and
  comments. No change means no write.

## PRDs

- Files `prd/<area>/<name>.md`. Title from the first `# ` heading without `PRD:`. Status line
  `> Status: <value> · <detail>` within the first 20 lines, following `>` lines belong to the detail.
- Values: `geplant` → planned, `umgesetzt (YYYY-MM-DD)` → merged, `verworfen (YYYY-MM-DD)` → discarded,
  anything else → unknown. After a merge the detail starts with `Kosten: <x.xx> USD · Modell: <model>`.
- A PRD that a run moved into its worktree is read from there while the main folder lacks it.
- Branch: `feat/<name>` if it exists, the base branch for merged PRDs.

### Status (`c`)

Only for PRDs without run (`<prd> has a run. Discard it first with X.`) and with a status line. Choices
planned, merged, discarded, the current one marked `(current)`. The current choice changes nothing. Merged and
discarded get today's date. Only the value before the first `·` changes.

- Untracked PRD: file changes, no commit.
- Tracked PRD: asks `Commit <path>? (y/n)`, then commits that file alone with
  `docs(prd): mark <name> as planned|done|discarded`, without trailer. Requires the main folder on the base
  branch and no uncommitted changes in the file.

## UI

Three columns: left and right `max(20, width/5)`, center the rest. The center holds the terminal box and
below it the details pane (`max(8, height/4)`, at most half; `z` zooms it to two thirds). The last line is the
footer: input, question, error, note, or key help with the Claude limits at the right end.

### Keys

1. Every global action has an `alt` key that works everywhere, inside a terminal too.
2. Outside a terminal, `alt` is optional unless the focused pane uses the plain key.
3. A key means the same in every pane: `j/k` move, `l`/`enter` open, `h` back.

Focus order for `alt+←/→` and `tab`: Projects → center → details pane → Explorer. Keys go to the terminal while
the center has the focus and a tab.

### Panes

- **Projects:** two lines per project: name and number of open tabs, then branch with `↑n ↓n` or `· local`
  or `no git`. Git info refreshes every 5 s. The selected project has a bar on the border.
- **Center:** terminal tabs (PTY + x/vt emulator) per project, they keep running while another project is
  selected. A project without tab starts `claude`. `alt+t` opens `$SHELL`. Window titles become tab titles,
  workflow tabs have fixed labels like `create`. Closing sends SIGHUP and kills after 2 s.
- **Details pane,** tab `Details` (default): with the PRDs tab active and a PRD selected, the PRD: without run
  its title, file, created date, status and branch, cost, model; with a run the run (phase, queue position,
  branch → base, port, cost, dev server, error, note), its steps with mark, duration and cost, and the live
  log of the current step. Otherwise the project: base branch, trust, prompts present/missing, hooks,
  knowledge (file → steps), allowed tools. Tab `Docker`: compose services in file order plus other
  containers of the project (label `com.docker.compose.project.working_dir`), with state, status, ports,
  image, age; `l` follows `docker logs -f --tail 200` in a tab. Docker refreshes on `docker events` and
  every 10 s.
- **Explorer,** tab `PRDs` (default): two lines per PRD (phase symbol and `<area>/<name>`, then phase and
  branch), open PRDs by label, then `▸ n done · x.xx USD` which `l` unfolds. PRDs refresh every 5 s, runs
  every second. Tab `Files`: file tree without `.git`, `l` opens folders or files in `$VISUAL`/`$EDITOR`/`vi`
  as a tab.

Phase symbols: `○` planned, `◌` waiting, `●` setup/implement/review/merge, `◐` merge-queue, `✓` merged,
`■` aborted, `✗` error/discarded.

### Live log

Step logs are JSONL. The view reads the last 256 KiB of the current step's log every second and shows one
line per content: runner lines, the first line of Claude texts, tool calls with their main argument
(`file_path`, `command`, `pattern`, `path`, `description`, `prompt`), tool errors, denied tools, subagents
(indented), the result (`result ok · n turns · s · USD`) and stderr. Lines get cut at 160 characters.

### Limits

Runners save every `rate_limit_event` to `$XDG_STATE_HOME/lazydev/usage.json`. The footer shows
`5h n% · 7d n%`, a window past its reset as 0%, 80% and more in the focus color.

## Workflow

### New PRD (`n`)

Input `New PRD, what is it about?`. `enter` opens a center tab `create` running `claude "<create prompt>"`
interactively in the main folder. Empty input asks Claude to ask. The contract makes Claude write
`prd/<area>/<name>.md` with status `geplant` and nothing else.

### Run (`s`)

Steps in order, each recorded in the run state:

1. **setup:** `git worktree add -b feat/<prd> <worktree> <base>` (an existing worktree stays, an existing
   branch without worktree is an error). An untracked PRD moves into the worktree and gets committed as
   `docs(prd): add <prd>`. Then the setup hook.
2. **implement:** Claude with the PRD path. Success requires `## Ergebnis` `umgesetzt` and the PRD status
   `umgesetzt (date)` in the worktree. `abgebrochen` fails with the first open question.
3. **review:** Claude reviews and commits through the agent `committer`. The worktree must be clean.
4. **merge:** wait for the project's merge slot (phase merge-queue), stop the dev server, require a clean
   worktree, `git rebase <base>`; on conflict `rebase --abort` and Claude step merge (`bereit für
   Fast-Forward`, base must be an ancestor, worktree clean). Check hook; if red, Claude step merge with the
   error as context and the check again. Cost and implement model go into the PRD status line, committed as
   `docs(prd): record cost of <prd>`. Wait while the main folder is not on the base branch or has changes
   outside untracked `prd/` files and `.lazydev/` (phase merge-queue, note `main folder: …`, checked every
   5 s). `git merge --ff-only feat/<prd>` in the main folder, teardown hook, `git worktree remove` and
   `git branch -d`. Cleanup errors only become a note.

After implement, review and the Claude step merge, nothing under `.lazydev/` may differ from the base branch
in the worktree. A Claude step succeeds only with exit code 0, a last `result` event without error and its
check. Merges always run automatically.

- `s` on an existing run continues with the failed or next step. Merged, discarded, already running or
  waiting PRDs refuse with a message. `s` asks `Start|Continue <prd>? (y/n)`.
- At most 3 runners work at the same time. Above that, the PRD gets phase waiting. A runner that ends starts
  waiting PRDs in start order.
- Ports start at 3500, skipping ports of unmerged runs and ports something listens on (IPv4 and IPv6).
- `x` stops a runner (phase aborted) or aborts a waiting PRD.
- `X` asks, then stops the dev server, runs the teardown hook, puts a moved PRD back into the main folder with
  status `geplant`, removes worktree and branch with force and deletes the run state and logs.

### Claude call

`claude -p <prompt> --output-format stream-json --verbose --permission-mode acceptEdits --allowedTools
<built-in git and file tools + allowedTools> --agents <state>/agents.json` in the worktree, stdin
`/dev/null`, `GIT_EDITOR=true`. Each stream line gets a `ts` field in the log, stderr lines become
`{"type":"stderr"}`. Session, model and cost go into the step as they arrive. The agents `committer` and
`structural-completeness-reviewer` are built in.

### Hooks and dev server

Hooks are scripts from `config.yaml`, run in the worktree with `LAZYDEV_PROJECT`, `LAZYDEV_WORKTREE`,
`LAZYDEV_BRANCH`, `LAZYDEV_PORT`, `LAZYDEV_SLUG`. Output goes to the step log. Exit code other than 0 fails the
step. Without a script a hook does nothing. The slug is the PRD name in lower case with `_` for other
characters, at most 40 bytes.

`D` starts `hooks.dev` detached in the worktree of a PRD with a run, with the hook variables and `PORT`, or
stops it (SIGTERM to its group, SIGKILL after 5 s). Its log is `runs/<id>/<prd>.dev.log`. The merge and `X`
stop it.

## Processes

- A runner is `lazydev run <id> <dir> <prd>` in its own session (`setsid`), stdout/stderr to
  `<prd>.runner.log`. Before spawning, lazydev copies its executable to `$XDG_STATE_HOME/lazydev/bin/lazydev`,
  so `go run` builds work and running runners keep their code.
- A run counts as running while its PID lives and `/proc/<pid>/cmdline` is `lazydev run …`. An active phase
  without a living runner shows as aborted.
- Locks are `flock` files in `locks/`: `start` while counting and spawning runners, `<id>.merge` per project.
- On SIGTERM a runner ends its context first, so the step records an abort, then signals its process group.

## Data

| Path | Content |
|---|---|
| `$XDG_STATE_HOME/lazydev/runs/<id>/<prd>.json` | Run state: projectDir, prd, branch, base, worktree, port, slug, phase, pid, startedAt, steps (name, sessionId, model, exitCode, ok, costUsd, startedAt, endedAt), error, note |
| `…/runs/<id>/<prd>.<step>.jsonl` | Step log: stream-json with `ts` and runner lines `{"type":"lazydev","time","text"}` |
| `…/runs/<id>/<prd>.runner.log`, `.devpid`, `.dev.log` | Runner output, dev server PID and folder, dev server output |
| `$XDG_STATE_HOME/lazydev/trust/<id>.json`, `usage.json`, `agents.json`, `bin/` | Trust, limits, agents, runner binary |
| `$XDG_DATA_HOME/lazydev/worktrees/<id>/<prd>/` | Worktrees |

State files are written atomically. Merged runs keep their state until discarded.

## CLI

`lazydev` starts the TUI. `start`, `stop`, `discard [--yes]`, `trust` take a project ID or path from the list;
`run` is internal. See `lazydev help`.
