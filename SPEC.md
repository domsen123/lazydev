# lazydev specification

Binding description of lazydev's behavior. Code, README.md and README.de.md follow it. Change it first.

## Goal

One terminal UI for the PRD workflow with Claude Code: write a PRD with Claude, implement and review it
without a human in an isolated worktree, merge it by fast-forward, and see everything live.

Not goals: remote runs, pull requests, issue trackers, editing PRDs inside lazydev.

## Projects

- The project list lives in `$XDG_CONFIG_HOME/lazydev/projects.json` (`{"projects": [<absolute path>]}`).
  `a` adds a folder (`~` expands, `Tab` completes folders), `d` removes it after `y`. Removing closes the
  project's terminals and keeps the folder.
- Adding a project, also one already in the list, runs `.lazydev/` init (see below).
- The project ID is the folder name. Projects that share a folder name get `-<first 6 hex of sha1(path)>`.
  IDs name state and worktree folders and are independent of the list order.
- `settings.json` holds:
  - `theme`: the color theme by name (`Default`, `Tokyo Night`).
  - `allowedTools`: rights for every Claude step of all projects, same syntax as in `config.yaml`, e.g.
    `Bash(head:*)`. Without the field there are no extra rights.
  - `permissionMode`: `acceptEdits` or `auto` for every Claude step of the runner. Empty or missing means
    `acceptEdits`. Any other value fails the Claude step before Claude starts with
    `settings.json: unknown permissionMode "<value>", allowed: acceptEdits, auto`. `s` continues at that
    step after a fix.
  - `discord`: the webhook of the runner's notifications (see Discord notifications),
    `{"enabled": bool, "url": string, "projects": {"<absolute project path>": {"enabled": bool, "url": string}}}`.
    Projects stand under their path like in `projects.json`, not under their ID. `enabled` is the global
    switch, missing or `false` means no project posts. A project without entry or without `enabled` is on,
    `false` mutes this project only. The URL is the project URL if not empty, otherwise the global URL.
    `.lazydev/config.yaml` has no Discord field, no URL lands in a repository. Removing a project with `d`
    keeps its entry.

  lazydev writes `settings.json` with the rights `0600`, also when it saves the theme. An existing file with
  other rights gets `0600` with the next save.

  lazydev writes no defaults into `settings.json`. A broken file gives the defaults: `acceptEdits` and no
  extra rights. Every Claude step reads the file at its start, so a change applies from the next step on,
  also in a running runner.

## `.lazydev/`

Folder in the main folder of a project, versioned with it. Nothing can be overridden locally.

| File | Content |
|---|---|
| `config.yaml` | `baseBranch` (default `main`), `knowledge`, `allowedTools`, `hooks` (`setup`, `teardown`, `check`, `dev`) |
| `<step>.md` | Prompt of `create`, `implement`, `review`, `merge`, `unblock` |

- Unknown fields in `config.yaml` are ignored, e.g. `preset` of older versions. Broken YAML is an error shown in
  the details pane, runs refuse to start.
- **Init** creates missing files only: `config.yaml` (empty, or `baseBranch` when the repo has neither `main`
  nor `master` but a checked out branch) and the five prompts of the built-in simple workflow. It never
  overwrites. Adding a project, opening the wizard, `n`, `f` and writing a prompt in the wizard run it, so a
  deleted `.lazydev/` comes back.
- The workflow reads prompts from `.lazydev/<step>.md` only. A missing prompt fails the step with
  `.lazydev/<step>.md is missing, press i to add it`.
- lazydev commits nothing under `.lazydev/` by itself. The commit of the git cockpit takes all changes,
  `.lazydev/` included.

### Prompt

A prompt consists of, separated by blank lines:

1. Header `# lazydev <step>: <argument>`. Without argument `<branch> → <base>`. Logs show it.
2. Knowledge block `## Pflichtlektüre` listing the files of `knowledge.all` and `knowledge.<step>`, without
   duplicates. Each file must exist where Claude runs (main folder for create, worktree otherwise, unblock
   included).
3. `.lazydev/<step>.md` without frontmatter, with `$ARGUMENTS`, `$BASE`, `$BRANCH` replaced.
4. The built-in contract of the step (`internal/prompts/contract/`). It has priority, a project cannot change
   it.

Arguments: create gets the user's description, implement and unblock the PRD path, review none, merge the
branch plus an optional error context. Prompts, contracts and PRD status values are German.

### Trust

`hooks` and `allowedTools` run code or widen Claude's rights. Their SHA-256 over the YAML of just these two
sections is trusted per project ID in `$XDG_STATE_HOME/lazydev/trust/<id>.json`. Comments and other fields do
not change it. Without both sections there is nothing to trust. `s` on an untrusted project asks
`.lazydev/config.yaml sets <hooks, n allowed tools>. Trust it and start <prd>? (y/n)`. `lazydev trust`
prints both sections and asks. Runners check trust before every start and again before every hook, so a
hook changed during a run does not run. `D` needs trust too. `X` on an untrusted project cleans up without
the teardown hook and reports `teardown hook not run`.

`allowedTools` in `~/.config/lazydev/settings.json` belong to the user. They need no trust and are not part
of the hash.

### Wizard (`i`)

Pages: base branch (local branches without `feat/`) → knowledge files (tracked `.md` outside `prd/` and
`.lazydev/`, `space` marks) → steps per file (`1`–`5` toggle create, implement, review, merge, unblock, new
files get create and implement) → prompts (`o` opens one in the editor as a center tab and hides the wizard until the
editor ends, `r` resets it to the simple workflow, an existing one after `y`) → summary with the resulting `config.yaml`, `enter`
saves.

- Without knowledge in `config.yaml`, `CLAUDE.md`, `ARCHITECTURE.md` and `_ARCHITECTURE.md` are preselected.
- A file marked for all five steps goes under `all`.
- Saving writes `baseBranch` (left out for `main`) and `knowledge` only and keeps other fields, order and
  comments. No change means no write.

## PRDs

- Files `prd/<area>/<name>.md`. Title from the first `# ` heading without `PRD:`. Status line
  `> Status: <value> · <detail>` within the first 20 lines, following `>` lines belong to the detail.
- Values: `geplant` → planned, `umgesetzt (YYYY-MM-DD)` → merged, `verworfen (YYYY-MM-DD)` → discarded,
  `blockiert (YYYY-MM-DD)` → blocked, anything else → unknown. After a merge the detail starts with
  `Kosten: <x.xx> USD · Modell: <model>`.
- A blocked run ends the PRD in its worktree with the section `## Blocker`: first line `Schritt: implement` or
  `Schritt: merge`, a blank line, then the open questions as a list. A new blocker replaces the section up to
  the next `## ` heading or the end of the file.
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

Focus order for `alt+←/→` and `tab`: Projects → center → Explorer → details pane. Keys go to the terminal while
the center has the focus and its active tab is a terminal, unless the tab is in copy mode.

`alt+c` opens copy mode of the active terminal tab (see Copy mode). Plain `c` never does, rule 2 skips it like
`g`, because `c` changes the status in the PRDs tab.

lazydev turns on mouse reporting (cell motion). Mouse events act only in the terminal area of the active
terminal tab, elsewhere and while an overlay (menu, wizard, status picker, git cockpit, question, input) is
open lazydev ignores them. They never go to the program in the tab and do not clear the footer error or note.
The native selection of the terminal works with Shift held while dragging.

### Panes

- **Projects:** two lines per project: name and number of open tabs, then branch with `↑n ↓n` or `· local`
  or `no git`. Git info refreshes every 5 s. The selected project has a bar on the border.
- **Center:** terminal tabs (PTY + x/vt emulator) per project, they keep running while another project is
  selected. A project without tab starts `claude`. `alt+t` opens `$SHELL`. Window titles become tab titles,
  workflow tabs have fixed labels like `create`. Closing sends SIGHUP and kills after 2 s. `alt+c`, the mouse
  wheel and dragging open copy mode of a terminal tab (see Copy mode). Log tabs are the
  second kind of tab: titled `log <prd>`, they show the log of a run without a process and count in the tab
  number of the project. `alt+w` closes a log tab right away. A log tab closes by itself once its run is
  merged or its run state is gone (`X`), it stays open after error, block or abort. Opening the log of a run whose
  log tab is open focuses that tab.
- **Details pane,** tab `Details` (default): with the PRDs tab active, the runs of the selected project
  (title `Runs`), independent of the PRD selected in the Explorer. The list holds every run that is not
  merged (waiting, setup, implement, review, merge, merge-queue, error, blocked, aborted), ordered by start time and
  then by PRD name. A header row names the columns, each run has one row: phase symbol and PRD label, phase
  (waiting with `· #n in queue`), current step with duration (`–` without step), cost (`x.xx USD`), port,
  dev server (`● :<port>` while running, otherwise `–`), branch, and in the rest of the width the first line
  of the last entry of the current step's log, faint. In phase error and blocked the first line of the error
  replaces it, otherwise a note does, both in the warning color. Too narrow, the columns drop from the right:
  log line, branch, dev, port, cost, step. PRD and phase stay. Keys: `j/k`, `g/G` move, `l`/`enter` open the
  log tab, `s`, `x`, `X`, `D`, `f` act on the selected run like in the PRDs tab, `i` wizard, `z` zoom. Each project keeps
  its own cursor, which clamps when its run disappears. Without runs, and with the Files tab active, the
  pane shows the project (title `Project`): base branch, trust, prompts present/missing, hooks,
  knowledge (file → steps), allowed tools. Tab `Docker`: compose services in file order plus other
  containers of the project (label `com.docker.compose.project.working_dir`), with state, status, ports,
  image, age; `l` follows `docker logs -f --tail 200` in a tab. Docker refreshes on `docker events` and
  every 10 s.
- **Explorer,** tab `PRDs` (default): two lines per PRD (phase symbol and `<area>/<name>`, then phase and
  branch), open PRDs by label, then `▸ n done · x.xx USD` which `l` unfolds. PRDs refresh every 5 s, runs
  every second. `L` opens the log tab of the selected PRD, without run the error `<label> has no run.`, on the
  group row nothing. `f` opens the unblock dialog of a blocked run (see Blocked runs). Tab `Files`: file tree without `.git`, `l` opens folders or files in `$VISUAL`/`$EDITOR`/`vi`
  as a tab.

Phase symbols: `○` planned, `◌` waiting, `●` setup/implement/review/merge, `◐` merge-queue, `✓` merged,
`■` aborted, `✗` error/discarded, `⊘` blocked (yellow `#d7af00`). Blocked PRDs are neither active nor done,
they stay in the group of open PRDs.

The footer of the PRDs tab and of the run list shows `f unblock` after `D dev` only while the selected run is
in phase blocked.

### Log tab

Step logs are JSONL. A log tab shows every step of `setup`, `implement`, `review`, `merge` whose log file
exists, each after a separator `── <step> ──`. One entry per content: runner lines, Claude texts with all
lines, tool calls with their complete main argument (`file_path`, `command`, `pattern`, `path`,
`description`, `prompt`), tool errors with their first line, denied tools, the result
(`result ok · n turns · s · USD`) and stderr. Tool results do not show. Each entry starts with `HH:MM:SS `
and has the color of its kind: runner lines faint, errors in the warning color, the result in the OK color.
Subagent entries indent by two spaces. Text wider than the tab wraps, words wider than the tab break hard.
Following lines and wrapped parts indent to the text column.

The header shows the run, unfolded when the tab opens: phase symbol and phase with queue position,
`branch → base`, port, cost, dev server; then the steps with mark, duration and cost (`✓` done, `●` running,
`✗` failed, `○` not started); then the error (phase error or blocked) or note in the warning color, and a
separator. `z` folds it to
one line: phase, current step with duration, cost.

`j/k` and `↓/↑` scroll one line, `ctrl+d/u` and `pgdown/pgup` half the height, `g`/`home` to the top,
`G`/`end` to the end, counted in wrapped screen lines. At the end the view follows new lines. Scrolling up
keeps the position, `G` or scrolling to the end follows again. Keys never go to a terminal, global keys work
without `alt` unless the tab uses the key.

Every second the tab reads what the runner appended to each step log since the last read, up to the last
complete line. A file shorter than the read offset gets read again from the start.

### Copy mode

Every terminal tab has a copy mode, log tabs have none. It copies the content of the tab only, without
borders and neighbor columns, and joins wrapped lines.

- **Open:** `alt+c` opens it for the active terminal tab of the selected project and focuses the center, from
  any pane. Without a terminal as active tab the error is `The active tab is not a terminal.`. `alt+c` in copy
  mode leaves it like `esc`.
- **Snapshot:** opening takes a snapshot of all lines: the scrollback (oldest first, up to 10 000 lines), then
  the lines of the screen. In the alt screen it takes the screen lines only. Copy mode shows and copies the
  snapshot only. The program keeps running, its output goes into the emulator and shows after leaving.
- **Start:** the view shows the end of the snapshot, the screen as before. The mark stands on the line of the
  cursor, with a hidden or invalid cursor on the last line.
- **View:** the mark is the whole line in reverse video over the full width of the tab. The cursor of the
  program does not show. The tab label ends with ` [copy]`. The footer shows
  ` j/k move · ctrl+d/u page · g/G top/end · v select · y copy · esc leave · alt+←/→ pane · alt+shift+←/→ tab · alt+w close · alt+m menu · alt+q quit`.
- **Keys:** `j`/`↓` and `k`/`↑` one line, `ctrl+d`/`pgdown` and `ctrl+u`/`pgup` half the height, `g`/`home`
  the first line, `G`/`end` the last line. The view scrolls along so the mark stays visible. `v` sets the anchor
  on the line of the mark, a second `v` drops the selection. All lines between anchor and mark, both
  included, are selected and show in reverse video. The keyboard selects whole lines only. `y` copies the
  selection, without one the line of the mark, and leaves copy mode. `esc` and `q` leave without copying.
  No key goes to the program. Global `alt` keys work, rule 2 does not apply, other keys do nothing.
- **State of the tab:** copy mode stays with the tab while the focus or the tab changes, until it is left.
  When the program ends, the tab closes as before and copy mode with it. A resize keeps the snapshot and
  clamps mark, anchor and view to the valid range, lines wider than the tab are cut.
- **Wheel:** up scrolls 3 lines up, outside copy mode it opens copy mode first (snapshot as with `alt+c`).
  Down scrolls 3 lines down. The mark moves with the view and keeps its row in the window. Copy mode opened
  by the wheel ends when wheel down reaches the end without a selection. Opened by `alt+c` it ends only with
  `esc`, `q`, `alt+c` or `y`.
- **Mouse:** a click in the terminal area focuses the center. Pressing the left button opens copy mode if it
  is not open and sets the start of the selection on the cell under the mouse, moving with the button held
  sets the end. The selection is exact to the cell like in a terminal: from the start cell to the end of its
  line, whole lines between, from the start of the line to the end cell. Start and end may be swapped. With
  the mouse above or below the terminal area while dragging, the view scrolls one line in that direction per
  motion event and the end of the selection follows. Left and right the column clamps to the area.
  Releasing copies the selection and removes it. Copy mode opened by the mouse ends then, otherwise it stays.
  Start and end on the same cell copy nothing. Pressing the mouse drops a keyboard selection, `v` drops a
  mouse selection.
- **Text:** without ANSI styles, trailing spaces removed, lines separated by `\n`, no `\n` after the last line.
  Cells of wide characters count once. x/vt keeps no soft wraps: a line that reaches the last column of the
  width the tab had at the snapshot (width in cells without trailing empty cells ≥ tab width) gets the next
  line appended without `\n`, in the scrollback and on the screen. After a resize of the tab the rule can be
  wrong for older lines.
- **Clipboard:** always OSC 52 (`ansi.SetSystemClipboard`) to the terminal of lazydev, in one write.
  Additionally `clipboard.WriteAll` of `github.com/atotto/clipboard` when the platform supports it and neither
  `SSH_CONNECTION` nor `SSH_TTY` is set. Its error is ignored. Success shows the note `Copied 1 line.` or
  `Copied <n> lines.`, counted in lines of the copied text after joining. Empty text copies nothing and shows
  the error `Nothing to copy.`, a failed OSC 52 write the error `Copy failed: <err>`. There is no own size
  limit, some terminals limit OSC 52.

### Git cockpit (`alt+g`)

`alt+g` opens the modal `Git · <name>` for the main folder of the selected project, everywhere, inside a
terminal and the log tab too. The tab `Git` of the menu (`alt+m`) opens it too. Plain
`g` never opens it, `g` stays "to the top" in lists. While the menu, a question, the status picker, the
wizard, the project input, the PRD input or the remove question is open, `alt+g` opens nothing. Without a
project the error is `Add a folder first: a in the left column`, in a folder without git
`<name> is no git repository.` and no modal opens.

The modal is `min(width − 4, 64)` wide. lazydev reads its content once when it opens, it does not refresh:

- `Branch    <branch> → <upstream> ↑n ↓n`, without upstream `<branch> · no upstream`, with a detached HEAD
  `detached at <7 chars>`. `↑n` and `↓n` only show above 0.
- `Changes   clean` or `Changes   <n> files`, `<n>` counts the lines of `git status --porcelain`, untracked
  files included.
- The actions `Fetch     f`, `Pull      p`, `Push      P`, `Commit    c`. The selected one shows `> ` and is
  bold in the selection colors.
- `Recent commits` with up to 5 lines of `git log --oneline -5`, without commits `no commits`.

Keys: `j`/`down` and `k`/`up` move, `enter` runs the selected action, `f`, `p`, `P`, `c` run theirs, `esc`,
`q`, `ctrl+c` and `alt+g` close. Commit opens the input `Message: ` in place of the actions. Every printable
key goes into it, `f`, `p` and `q` too. `enter` commits, `esc` goes back to the actions. An empty message or
one of spaces only starts nothing and shows the error `Enter a commit message.`. The footer shows
` j/k move · enter run · f fetch · p pull · P push · c commit · esc close`, in the input
` enter commit · esc back`. Errors and notes come first.

An action closes the modal and opens a center tab in the main folder with the fixed label `git fetch`,
`git pull`, `git push` or `git commit`. The tab runs `lazydev git <id> <dir> <action> [<message>]`, which
prints `$ git <args>` before each git call. Git reads from the PTY, so passphrase and credential prompts work
like in a shell. All actions act on the branch checked out in the main folder.

| Action | Git |
|---|---|
| Fetch | `git fetch` |
| Pull | `git pull --ff-only`. A divergence is an error of git |
| Push | `git push` with upstream, otherwise `git push -u origin <branch>`. Upstream means `git rev-parse --abbrev-ref --symbolic-full-name @{upstream}` succeeds. A missing remote `origin` is an error of git |
| Commit | `git add -A`, then `git commit -m <message>`. All changes, untracked files, `.lazydev/` and `prd/` included. The message goes to git unchanged, without trailer |

- Pull and push with a detached HEAD print `HEAD is detached, check out a branch first.` and end with 1.
  Fetch and commit run.
- Pull and push take `locks/<id>.merge` before git starts and free it after git, without waiting. While a
  runner holds it, they print `A PRD of <id> merges right now. Try again once the merge is done.` and end
  with 1.
- Nothing to commit gets no own check, git reports it as an error.
- Success: exit code 0, the tab closes, the footer note is `git <action>: done`.
- Failure: a blank line and `failed (exit <n>), press enter to close`. After `enter` the tab closes with
  `<n>` and the footer error is `git <action> failed (exit <n>)`.
- `alt+w` ends the tab with SIGHUP, the footer error is `git <action> stopped`.
- When a git tab ends, lazydev reads the git info of all projects right away.

### Menu (`alt+m`)

The menu is a box `Menu` of `min(width − 4, 72)` columns with two columns like the options of btop. The left
column is 14 characters wide inside and holds the tabs `General`, `Discord`, `Git`, then a blank line and
`Quit`. After a `│` the right column holds the entries of the tab or its description. Below both stands a
hint line.

| Tab | Right column |
|---|---|
| `General` | Entry `Color theme  ‹ <name> ›`, below it `Colors of borders, focus and selection. lazydev saves the choice in settings.json.`, a blank line and `lazydev <version>`. With a newer release known the line is `lazydev <version> · <latest> available, alt+u updates.`, after the update `lazydev <version> · <tag> installed, restart lazydev.` (see Releases) |
| `Discord` | Entries `Notifications  ‹ on/off ›` (global), `Global URL  <url>`, `<project name>  ‹ on/off ›`, `URL for <project name>  <url>`, `Send test`. Without a selected project the two project entries drop out. Below them the description of the selected entry |
| `Git` | `Fetch, pull, push and commit the main folder of the selected project. Each action runs in a center tab. Also alt+g.` |
| `Quit` | `Ends lazydev and its terminal tabs. Runners keep working in the background.` |

Descriptions of the Discord entries:

- Notifications: `Runners post to Discord when a PRD is merged, fails, gets blocked, gets aborted or waits for the main folder.`
- Global URL: `Webhook URL for all projects. Stored in settings.json, never in a repository.`
- Project switch: `Off mutes this project only.`
- Project URL: `Replaces the global URL for this project. Empty uses the global URL.`
- Send test: `Posts a test message to the URL of this project now, also while off.`

URLs show shortened to `…/<last 4 characters>`. Without URL the global URL shows `not set`, the project URL
`global`.

Keys in the left column: `↑/↓`, `j/k` and `tab` move and wrap around. `enter`, `→` and `l` open: `General`
and `Discord` go to the right column, `Git` closes the menu and opens the git cockpit, `Quit` ends lazydev.
`esc`, `q` and `alt+m` close the menu. Hint: `↑/↓ select · enter open · esc close`.

Keys in the right column: `↑/↓`, `j/k` and `tab` move and wrap around. `←/→` change switches and the theme.
`enter` and `space` flip switches, go to the next theme, open the URL input or send the test. `esc` and `h`
go back to the left column, `q` and `alt+m` close the menu. Hint: `↑/↓ select · ←/→ change · enter edit · esc back`.

- The theme applies right away and goes into `settings.json`.
- Every change in the tab `Discord` reads `settings.json`, changes the field and writes it right away. An error
  while writing goes into the footer.
- `enter` on a URL opens the empty input `Webhook URL: ` in place of the entries, hint `enter save · esc
  cancel`. `enter` saves, `esc` cancels. An empty input removes the URL. An input that does not start with
  `https://` after trimming spaces saves nothing and shows the error `Enter an https URL.`.
- `Send test` posts the embed `Test from lazydev` (color `0x3498db`, field `Project` with the project name,
  `–` without project) to the URL of the selected project, without project to the global URL. One attempt,
  no retry, 10 s timeout, also while the switches are off. It runs in the background. Without URL the error
  is `No webhook URL set.`. Success shows the note `discord: test sent`, failure the error
  `discord: <error>`.

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
   `umgesetzt (date)` in the worktree. `abgebrochen` with points under `## Offene Fragen` blocks the run (see
   Blocked runs), `abgebrochen` without them fails with `implement aborted`. `## Hinweise` holds remarks
   without a decision and changes nothing.
3. **review:** Claude reviews and commits through the agent `committer`. The worktree must be clean.
4. **merge:** wait for the project's merge slot (phase merge-queue), stop the dev server. If the PRD is the
   only uncommitted file in the worktree, commit it as `docs(prd): resolve blocker of <prd>`. Require a clean
   worktree, `git rebase <base>`; on conflict `rebase --abort` and Claude step merge (`bereit für
   Fast-Forward`, base must be an ancestor, worktree clean; `abgebrochen` with points under `## Offene Punkte`
   blocks the run, without them it fails with `merge aborted`). Check hook; if red, Claude step merge with the
   error as context and the check again. Cost and implement model go into the PRD status line, committed as
   `docs(prd): record cost of <prd>`. Wait while the main folder is not on the base branch or has changes
   outside untracked `prd/` files and `.lazydev/` (phase merge-queue, note `main folder: …`, checked every
   5 s). `git merge --ff-only feat/<prd>` in the main folder, teardown hook, `git worktree remove` and
   `git branch -d`. Cleanup errors only become a note.

After implement, review and the Claude step merge, nothing under `.lazydev/` may differ from the base branch
in the worktree. A Claude step succeeds only with exit code 0, a last `result` event without error and its
check. Merges always run automatically.

- `s` on an existing run continues with the failed, blocked or next step. Merged, discarded, already running
  or waiting PRDs refuse with a message. A run whose PRD in the worktree still says `blockiert` refuses with
  `<prd> is blocked. Resolve it with f first.`. `s` asks `Start|Continue <prd>? (y/n)`.
- At most 3 runners work at the same time. Above that, the PRD gets phase waiting. A runner that ends starts
  waiting PRDs in start order.
- Ports start at 3500, skipping ports of unmerged runs and ports something listens on (IPv4 and IPv6).
- `x` stops a runner (phase aborted) or aborts a waiting PRD.
- `X` asks, then stops the dev server, runs the teardown hook, puts a moved PRD back into the main folder with
  status `geplant` and without `## Blocker`, removes worktree and branch with force and deletes the run state
  and logs.

### Blocked runs

Claude aborts implement or the Claude step merge on purpose when the PRD leaves a decision open. Such a run
needs a decision of the user, not another attempt.

- **When:** exit code 0, a last `result` without error, `## Ergebnis` `abgebrochen` and at least one point
  under `## Offene Fragen` (implement) or `## Offene Punkte` (merge) that is not „keine“. Points are the
  non-empty lines of the section without a leading `- `. Everything else stays phase error: another exit code,
  missing `## Ergebnis`, `abgebrochen` without points, red checks, the status check, hooks, git errors. Review
  never blocks.
- **Run:** the runner sets the status line of the PRD in the worktree to `blockiert (<today>)`, writes
  `## Blocker` with all points and commits the PRD alone, without Claude and trailer, as
  `docs(prd): record blocker of <prd>`. Other uncommitted changes of the step stay uncommitted. The step
  ends with `ok: false`, the run gets phase blocked, error `<step> blocked: <first point>` and PID 0. If writing
  or the commit fails, the run gets phase error with that error.
- **`f`:** in the PRDs tab and in the run list on a run in phase blocked, otherwise the error
  `<prd> is not blocked.`. It runs `.lazydev/` init, builds the prompt of the step `unblock` with the PRD path
  relative to the worktree as `$ARGUMENTS` and opens a center tab `unblock` running
  `claude "<unblock prompt>" --agents <state>/agents.json` interactively in the worktree.
- **Dialog:** the contract makes Claude ask the questions of `## Blocker` numbered with options A to D, wait
  for the answers and work them into the PRD. Then it removes `## Blocker` and sets the status line to
  `geplant` after `Schritt: implement`, to `umgesetzt (<today>)` after `Schritt: merge`. It changes only the
  PRD, commits nothing and ends with `Weiter in lazydev mit s.`. The dialog runs outside the runner, the
  next step checks `.lazydev/` anyway.
- **Continue:** nothing starts by itself, `s` continues with the blocked step. While the PRD in the worktree
  still says `blockiert`, the TUI refuses (see Run) and a runner ends with `<prd> is still blocked` without
  changing run state, phase or log. After implement the review commits the changed PRD with the rest, before
  the merge the runner commits it (see merge).

### Discord notifications

The runner posts to the Discord webhook of the project (see `discord` in `settings.json`), the TUI does
not. Notifications come without a running TUI too. The runner reads `settings.json` at every event, so a
change in the menu applies to running runners too.

| Event | When | Title | Color | Log |
|---|---|---|---|---|
| `merged` | the PRD is merged | `✓ Merged: <area>/<name>` | `0x2ecc71` | merge |
| `failed` | a step fails (phase error) | `✗ Failed: <area>/<name>` | `0xe74c3c` | the failed step |
| `blocked` | a step blocks (phase blocked), no `failed` then | `⊘ Blocked: <area>/<name>` | `0xe67e22` | the blocked step |
| `aborted` | `x` ends a running runner (phase aborted) | `■ Aborted: <area>/<name>` | `0x95a5a6` | the aborted step |
| `main folder` | the merge starts to wait for the main folder | `◐ Waiting for main folder: <area>/<name>` | `0xf1c40f` | merge |

- One notification per waiting phase for the main folder, when the waiting starts. A change of the reason
  posts nothing.
- A waiting PRD that `x` aborts without runner posts nothing, nor do errors before the first step (claude
  missing, trust, preparing the run).
- The payload is `{"username":"lazydev","embeds":[{"title":…,"color":…,"fields":[…],"timestamp":<RFC 3339>}]}`.
  The fields are all `inline: true`, in this order: `Project` (project ID), `Branch` (`<branch> → <base>`),
  `Cost` (`x.xx USD`), `Duration` (since the start of the run, `1h 2m 3s` without leading zero parts, e.g.
  `4m 5s`), `Model` (implement model, `–` without). The embed holds no error text. `blocked` adds the field
  `Blocker` (`inline: false`) with all open questions, one per line, cut to 1024 characters with `…` at the end.
- HTTP `POST` with `Content-Type: application/json`, 10 s timeout per attempt, success is a 2xx status. One
  attempt plus 3 retries 5 s apart, every error retries, 4xx too. The delivery runs in the runner and delays
  it by about 55 s at most. An abort posts with its own context.
- Without URL or with a switch off nothing posts and the log gets nothing. Success logs
  `Discord notification sent: <event>`, failure after the last attempt `Discord notification failed: <error>`.
  Phase, error and note of the run stay, the step does not fail.

### Claude call

`claude -p <prompt> --output-format stream-json --verbose --permission-mode <mode> --allowedTools <list>
--agents <state>/agents.json` in the worktree, stdin `/dev/null`, `GIT_EDITOR=true`. `<mode>` is
`permissionMode` from `settings.json`. `<list>` holds the built-in git and file tools, then `allowedTools`
from `settings.json`, then `allowedTools` from `config.yaml`. Duplicates drop out, the first one stays. In
mode `auto` the list goes to Claude unchanged. Before the line `$ claude …` the step log gets
`Permissions: <mode>, <n> allowed tools`, `<n>` counts the list without duplicates. Each stream line gets a
`ts` field in the log, stderr lines become `{"type":"stderr"}`. Session, model and cost go into the step as
they arrive. The agents `committer` and `structural-completeness-reviewer` are built in.

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
- `lazydev git <id> <dir> <action> [<message>]` runs in the terminal tab of the git cockpit. Pull and push
  take `locks/<id>.merge` without waiting.
- On SIGTERM a runner ends its context first, so the step records an abort, then signals its process group.

### Releases

- `internal/version.Version` is the version of the binary, `dev` by default. `make build` and `make install`
  set it to `git describe --tags --always --dirty`, GoReleaser to the tag, e.g. `v0.1.0`.
- Pushing a tag `v*` starts the GitHub Action `release`: `make test`, then GoReleaser v2 builds `linux/amd64`
  and `linux/arm64` with `CGO_ENABLED=0` and creates the release with
  `lazydev_<version without v>_linux_<arch>.tar.gz` (`lazydev`, `LICENSE`, `README.md`, `README.de.md`) and
  `checksums.txt` (SHA-256). Tags follow `vMAJOR.MINOR.PATCH` without suffix.
- The release notes come from the commits since the previous tag, grouped into `Features` (`feat`), `Fixes`
  (`fix`), `Performance` (`perf`) and `Other`. `docs`, `test`, `chore`, `ci`, `style` and `build` drop out.
  There is no `CHANGELOG.md`.
- The repository is public. `install.sh` in the repository root downloads a release with `curl` from
  `https://github.com/domsen123/lazydev/releases/download/<tag>/`, the newest one (`tag_name` of
  `https://api.github.com/repos/domsen123/lazydev/releases/latest`) or the tag of its first argument. It
  needs `curl`, `tar`, `sha256sum` and `uname`, Linux and `amd64` or `arm64`, and checks the checksum. It installs `$PREFIX/bin/lazydev` (`PREFIX` defaults to
  `$HOME/.local`) and prints `Installed lazydev <tag> to <path>. Restart lazydev to use it.`, plus
  `Add <dir> to PATH.` when the folder is not in `PATH`. Errors print one line `install.sh: …` to stderr and
  end with 1. Running it again updates. Running runners keep their copy in `bin/`.
- Update check: once at the start of the TUI, in the background, only when `Version` has the format
  `vMAJOR.MINOR.PATCH` without suffix. It reads `tag_name` of
  `https://api.github.com/repos/domsen123/lazydev/releases/latest` with a 10 s timeout. A numerically higher
  tag sets the footer note `lazydev <latest> available, you run <version>. alt+u updates.` in yellow unless an
  error or note shows already, the version line of the menu tab `General`, and puts `alt+u update <latest> ·`
  in yellow in front of the key help until the update is done. No network, timeout, a status other than 200
  or unreadable output show nothing.
- Update (`alt+u`, outside a terminal also `u`): with a newer release known it installs it in the background,
  the footer note is `Updating lazydev to <latest> …`. It downloads `checksums.txt` and the archive of the
  architecture, checks the SHA-256, takes `lazydev` from the archive and replaces the binary of the running
  TUI (`os.Executable`, symlinks resolved) atomically through a temporary file in its folder, mode 0755. The
  timeout is 2 min. Success: the yellow note `lazydev <tag> installed. Restart lazydev to use it.`, the TUI
  keeps running the old code. Failure: the footer error `update: <error>`, the binary stays, `alt+u` tries
  again. Without a known newer release the note is `No newer release of lazydev known.`, after the update
  `alt+u` repeats the installed note. Running runners keep their copy in `bin/`, runners started later
  copy the new binary.
- `lazydev update` does the same on the command line: `lazydev <version> is no release build, update it from
  source` for builds without release version (exit 1), `lazydev <version> is the newest release.` without a
  newer one, else `Installed lazydev <tag> to <path>. Restart lazydev to use it.`.

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
`run` and `git` are internal. `update` installs the newest release over the binary (see Releases). `version`, `--version` and `-v` print `lazydev <version>` and end with 0. See
`lazydev help`.
