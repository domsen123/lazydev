# lazydev specification

Binding description of lazydev's behavior. Code, README.md and README.de.md follow it. Change it first.

## Goal

One terminal UI for the PRD workflow with Claude Code: write a PRD with Claude, implement and review it
without a human in an isolated worktree, merge it by fast-forward, and see everything live.

Not goals: remote runs, pull requests, issue trackers, editing PRDs inside lazydev.

## Projects

- The project list lives in `$XDG_CONFIG_HOME/lazydev/projects.json` (`{"projects": [<absolute path>]}`).
  `a` adds a folder (`~` expands, `Tab` completes folders), `d` removes it after `y`. Removing closes the
  project's terminals and keeps the folder. `lazydev add` and `lazydev remove` do the same without the TUI
  (see CLI). Every change reads the list right before writing it, and the TUI takes the list from the file
  every 5 s, so a running TUI shows projects the CLI added and drops removed ones with their terminals.
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
  - `skill`: `false` stops the TUI from writing the Claude Code skill at its start (see Claude Code
    skill). Missing or `true` means it writes it.
  - `autoUnblock`: `{"<absolute project path>": true}`, the projects whose blocked runs Claude resolves
    itself (see Blocked runs). Missing means off, switching off removes the entry. `lazydev auto-unblock`
    and the menu set it.

  lazydev writes `settings.json` with the rights `0600`, also when it saves the theme. An existing file with
  other rights gets `0600` with the next save.

  lazydev writes no defaults into `settings.json`. A broken file gives the defaults: `acceptEdits` and no
  extra rights. Every Claude step reads the file at its start, so a change applies from the next step on,
  also in a running runner.

## `.lazydev/`

Folder in the main folder of a project, versioned with it. Nothing can be overridden locally.

| File | Content |
|---|---|
| `config.yaml` | `baseBranch` (default `main`), `knowledge`, `allowedTools`, `hooks` (`setup`, `teardown`, `check`, `dev`), `databases` (see Databases) |
| `<step>.md` | Prompt of `create`, `implement`, `review`, `merge`, `unblock` |

- Unknown fields in `config.yaml` are ignored, e.g. `preset` of older versions. Broken YAML is an error shown in
  the details pane, runs refuse to start.
- **Init** creates missing files only: `config.yaml` (empty, or `baseBranch` when the repo has neither `main`
  nor `master` but a checked out branch) and the five prompts of the built-in simple workflow. It never
  overwrites. Adding a project, opening the wizard, `n`, `f` and writing a prompt in the wizard run it, so a
  deleted `.lazydev/` comes back.
- The workflow reads prompts from `.lazydev/<step>.md` only. A missing prompt fails the step with
  `.lazydev/<step>.md is missing, lazydev add <project folder> or i in the TUI adds it`.
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

`hooks`, `allowedTools` and `databases` run code, widen Claude's rights or create and drop databases. Their
SHA-256 over the YAML of just these three sections is trusted per project ID in
`$XDG_STATE_HOME/lazydev/trust/<id>.json`. Comments and other fields do not change it. Without all three
sections there is nothing to trust. `s` on an untrusted project asks
`.lazydev/config.yaml sets <hooks, n allowed tools, n databases>. Trust it and start <prd>? (y/n)`, e.g.
`.lazydev/config.yaml sets setup scripts/setup.sh, 2 databases. Trust it and start login-form? (y/n)`.
`lazydev trust` prints the three sections and asks. Runners check trust before every start and again before
every hook and every database action, so a hook or a database list changed during a run does not run. `D`
needs trust too. `X` on an untrusted project cleans up without the teardown hook and reports
`teardown hook not run`, drops no database and reports
`databases not dropped, databases in .lazydev/config.yaml are not trusted`.

`allowedTools` in `~/.config/lazydev/settings.json` belong to the user. They need no trust and are not part
of the hash.

### Wizard (`i`)

Six pages, numbered `1/6` to `6/6`: base branch (local branches without `feat/`) → knowledge files (tracked
`.md` outside `prd/` and `.lazydev/`, `space` marks) → steps per file (`1`–`5` toggle create, implement,
review, merge, unblock, new files get create and implement) → databases (see below) → prompts (`o` opens one
in the editor as a center tab and hides the wizard until the editor ends, `r` resets it to the simple
workflow, an existing one after `y`) → summary with the resulting `config.yaml`, `enter` saves. Without
marked knowledge files `enter` goes from knowledge straight to databases, and `backspace` on databases goes
back to knowledge.

- Without knowledge in `config.yaml`, `CLAUDE.md`, `ARCHITECTURE.md` and `_ARCHITECTURE.md` are preselected.
- A file marked for all five steps goes under `all`.
- Saving writes `baseBranch` (left out for `main`), `knowledge` and `databases` only and keeps other fields,
  order and comments. No change means no write. An unchanged `databases` stays untouched, an empty list
  removes the key. A changed `databases` changes the trust hash, so the next `s` asks for trust.

**Page Databases:** heading `Databases`, hint `Each worktree gets an empty database. Migrate in the setup
hook.` When the wizard opens it reads `databases` of `config.yaml` and the services of the default compose
file of the main folder plus every file from `compose` of the entries (`docker compose [-f <file>] config
--format json`). A service is Postgres if its image name without registry and tag is `postgres` or ends in
`/postgres`, `/postgis`, `/pgvector` or `/timescaledb` (e.g. `postgis/postgis`, `pgvector/pgvector`,
`timescale/timescaledb`), or if its `environment` has a key starting with `POSTGRES_`.

- Rows: each Postgres service as `<service>  <image>`, sorted by name, the default file first. Below a
  service, indented, its entries as `→ <file> <key>`. Entries whose service no compose file has as Postgres
  service show below their service with ` (not in compose file)` and stay. An entry found under a file from
  `compose` keeps its `compose`, new entries of services of the default file get none.
- Without compose file the page shows `No compose.yaml or docker-compose.yml in the main folder. Set
  compose: in config.yaml for other files.`, without Postgres service `No Postgres service in the compose
  file.`. If `docker compose config` cannot read a file, the page shows the first line of the error. `enter`
  goes on in every case.
- `a` on a service row opens the choice of the env file: all regular files in the main folder whose name
  starts with `.env` and that git ignores, sorted. Without one the wizard shows the error `No ignored .env
  file in the main folder.`. `enter` picks the file, then the choice of the key follows: all keys of the file,
  first the ones whose name contains `DATABASE`, `DB_NAME` or `_URL` (any case), otherwise in file order.
  `enter` adds the entry, `esc` in a choice goes back to the list without a change. An entry that exists
  already (same service, file and key) is not added twice.
- `d` on an entry row removes it from the wizard right away, saving happens in the summary.
- Keys: `j`/`k` select, `a` add, `d` remove, `enter` next, `backspace` back, `esc` closes the wizard. Footer
  ` ↑↓ select · a add · d remove · enter next · backspace back · esc cancel`, in a choice
  ` ↑↓ select · enter choose · esc back`.
- Saving writes the entries in the order of the page.

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

`alt+v` opens the Markdown viewer of the selection in the right column (see Markdown viewer), from anywhere,
inside a terminal and a log tab too. Rule 2 applies, plain `v` works outside a terminal because no pane uses
it. In copy mode `v` stays the selection, copy mode keys run before rule 2.

lazydev turns on mouse reporting (cell motion). Mouse events act only in the terminal area of the active
terminal tab or Markdown viewer, elsewhere and while an overlay (menu, wizard, status picker, git cockpit,
question, input) is open lazydev ignores them. They never go to the program in the tab and do not clear the
footer error or note.
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
  log tab is open focuses that tab. Markdown viewers are the third kind of tab: titled `md <file>`, they
  show a Markdown file rendered, without a process, and count in the tab number too (see Markdown viewer).
- **Details pane,** tab `Details` (default): with the PRDs tab active, the runs of the selected project
  (title `Runs`), independent of the PRD selected in the Explorer. The list holds every run that is not
  merged (waiting, plan, setup, implement, resolve, review, merge, merge-queue, error, blocked, aborted), ordered by start time and
  then by PRD name. A header row names the columns, each run has one row: phase symbol and PRD label, phase
  (waiting with `· #n in queue`), current step with duration (`–` without step), cost (`x.xx USD`), port,
  dev server (`● :<port>` while running, otherwise `–`), branch, and in the rest of the width the first line
  of the last entry of the current step's log, faint. In phase error and blocked the first line of the error
  replaces it, otherwise a note does, both in the warning color. Too narrow, the columns drop from the right:
  log line, branch, dev, port, cost, step. PRD and phase stay. Keys: `j/k`, `g/G` move, `l`/`enter` open the
  log tab, `s`, `x`, `X`, `D`, `f` act on the selected run like in the PRDs tab, `i` wizard, `z` zoom. Each project keeps
  its own cursor, which clamps when its run disappears. Without runs, and with the Files tab active, the
  pane shows the project (title `Project`): base branch, trust, prompts present/missing, hooks,
  databases (`<service> → <file> <key>` per entry, `–` without), knowledge (file → steps), allowed tools.
  Its columns drop from the right too. Tab `Docker`: compose services in file order plus other
  containers of the project (label `com.docker.compose.project.working_dir`), with state, status, ports,
  image, age; `l` follows `docker logs -f --tail 200` in a tab. Docker refreshes on `docker events` and
  every 10 s.
- **Explorer,** tab `PRDs` (default): two lines per PRD (phase symbol and `<area>/<name>`, then phase and
  branch), open PRDs newest first by modification time of the file (label on ties), then `▸ n done · x.xx USD`
  which `l` unfolds, sorted the same way. PRDs refresh every 5 s, runs every second. `L` opens the log tab of the selected PRD, without run the error `<label> has no run.`, on the
  group row nothing. `f` opens the unblock dialog of a blocked run (see Blocked runs). Tab `Files`: file tree without `.git`, `l` opens folders or files in `$VISUAL`/`$EDITOR`/`vi`
  as a tab.

Phase symbols: `○` planned, `◌` waiting, `●` plan/setup/implement/resolve/review/merge, `◐` merge-queue, `✓` merged,
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
`branch → base`, port, cost, dev server; then, only when the run state has databases, the line
`databases <name> (<service>) · <name> (<service>)`; then the steps with mark, duration and cost (`✓` done, `●` running,
`✗` failed, `○` not started); then the error (phase error or blocked) or note in the warning color, and a
separator. `z` folds it to
one line: phase, current step with duration, cost.

`j/k` and `↓/↑` scroll one line, `ctrl+d/u` and `pgdown/pgup` half the height, `g`/`home` to the top,
`G`/`end` to the end, counted in wrapped screen lines. At the end the view follows new lines. Scrolling up
keeps the position, `G` or scrolling to the end follows again. Keys never go to a terminal, global keys work
without `alt` unless the tab uses the key.

Every second the tab reads what the runner appended to each step log since the last read, up to the last
complete line. A file shorter than the read offset gets read again from the start.

### Markdown viewer

`alt+v` opens the selection of the active tab of the right column of the selected project, whichever pane
has the focus: in tab `Files` the marked row, in tab `PRDs` the marked PRD at `<root>/<path>`, for a PRD that
a run moved into its worktree the file in the worktree. `l`/`enter` keep opening the editor.

- **Errors:** no tab opens and the footer shows the error. Without project
  `Add a folder first: a in the left column`. Tab `Files` on a folder or with an empty tree
  `Select a Markdown file in the Files tab.`. Tab `PRDs` on the group row or without PRDs
  `Select a PRD in the PRDs tab.`. An extension other than `.md` or `.markdown` (any case)
  `<file> is no Markdown file.`. An unreadable file `Cannot read <file>: <first line of the error>`. `<file>` is
  the base name.
- **Tab:** a new center tab of the selected project, appended on the right, becomes active and the center
  gets the focus. Title `md <file>`, e.g. `md README.md`. It counts in the tab number of the project. If a
  viewer of the same absolute path is open in the project, `alt+v` focuses it instead and does not read the
  file again.
- **Rendering:** Glamour (`github.com/charmbracelet/glamour`) with a fixed style per theme: `Default` the
  dark style, `Tokyo Night` the Tokyo Night style. No automatic style detection. Word wrap at the tab width.
  A theme change in the menu renders again. The tab shows the rendered Markdown only, without header. Lines
  wider than the tab are cut, shorter ones filled with spaces. If Glamour fails, the tab shows the raw text,
  tabs as four spaces, wrapped to the tab width.
- **Keys:** `j`/`↓` and `k`/`↑` one line, `ctrl+d`/`pgdown` and `ctrl+u`/`pgup` half the height,
  `g`/`home` to the top, `G`/`end` to the end. The viewer starts at the top and follows no end. `e` opens the
  file with `$VISUAL`/`$EDITOR`/`vi` as a new terminal tab, the viewer stays open. Keys never go to a terminal,
  other keys work as global keys (rule 2). `alt+w` closes the viewer right away. `alt+c` shows
  `The active tab is not a terminal.`, viewers have no copy mode.
- **Refresh:** every second lazydev checks modification time and size of the files of all viewers of all
  projects in the background. If one of them changed, it reads the file again and the tab renders again. The
  scroll position stays, clamped to the new end. Without change it does not read the file.
- **Gone file:** if the check does not find the file or cannot read it, the tab closes by itself with the
  note `<file> is gone, its viewer closed.` or `Cannot read <file>: <first line of the error>, its viewer closed.`.
  With several tabs at once the note of the last one wins.
- **Run:** a viewer that opened a PRD from the worktree of a run closes with that run like a log tab, once
  the run is merged or its run state is gone (`X`), without note. It never switches to the file in the main
  folder. Viewers of files in the main folder belong to no run.
- **Mouse:** the wheel in the content area of the active viewer (the terminal area) scrolls 3 lines, a left
  click there focuses the center. Nothing else, there is no selection.
- **Footer:**
  ` j/k scroll · ctrl+d/u page · g/G top/end · e edit · alt+←/→ pane · alt+shift+←/→ tab · alt+j/k project · alt+w close · t terminal · alt+g git · m menu · q quit`.
  The footers of the tabs `PRDs` and `Files` show `v view` after `l open`, the terminal footer `alt+v view`
  after `alt+c copy`.

### Copy mode

Every terminal tab has a copy mode, log tabs and Markdown viewers have none. It copies the content of the tab only, without
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
| `General` | Entries `Color theme  ‹ <name> ›` and, with a selected project, `Auto-unblock <project name>  ‹ on/off ›`. Below them the description of the selected entry: `Colors of borders, focus and selection. lazydev saves the choice in settings.json.` or `On: when a run of this project gets blocked, Claude answers the open questions itself, records them under ## Entscheidungen and the run goes on. Questions about data loss, security, public interfaces or costs still stop the run.`, a blank line and `lazydev <version>`. With a newer release known the line is `lazydev <version> · <latest> available, alt+u updates.`, after the update `lazydev <version> · <tag> installed, restart lazydev.` (see Releases) |
| `Discord` | Entries `Notifications  ‹ on/off ›` (global), `Global URL  <url>`, `<project name>  ‹ on/off ›`, `URL for <project name>  <url>`, `Send test`. Without a selected project the two project entries drop out. Below them the description of the selected entry |
| `Git` | `Fetch, pull, push and commit the main folder of the selected project. Each action runs in a center tab. Also alt+g.` |
| `Quit` | `Ends lazydev and its terminal tabs. Runners keep working in the background.` |

Descriptions of the Discord entries:

- Notifications: `Runners post to Discord when a PRD is merged, fails, gets blocked, gets unblocked by Claude, gets aborted or waits for the main folder.`
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

- The theme applies right away and goes into `settings.json`. The auto-unblock switch reads `settings.json`,
  flips `autoUnblock` of the selected project and writes it right away.
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

### Activity log

The TUI appends what the user does to `$XDG_STATE_HOME/lazydev/activity.jsonl`, one JSON object per
line, so a week of use can be read back and turned into a plan. `"activity": false` in `settings.json`
switches it off. Every line has `t` (RFC 3339) and `kind`:

| `kind` | When | Fields |
|---|---|---|
| `session` | TUI start and end | `text` `start` (with `version`) or `end` |
| `key` | a key outside a terminal tab | `key`, `pane` (`projects`, `center`, `explorer`, `details`), `tab` (`prds`, `files`, `details`, `docker`, `log`, `md`), `dialog` (`menu`, `menu-input`, `git`, `question`, `status`, `wizard`, `add`, `create`, `remove`), `project`, `prd`, `phase` |
| `mouse` | a mouse press, not the wheel | like `key`, `key` is the event (`left press`) |
| `terminal` | keys start going to a terminal tab, and when they stop | `text` `enter` or `leave`, `tab` `terminal:<program>`; `leave` carries `count` (keys sent) and `seconds` (active time: gaps over five minutes left out) |
| `action` | the footer text of an action (start, stop, discard, unblock, status, git) | `text`, `err`, `project` |
| `run` | the run poll shows a phase the poll before did not | `project`, `prd`, `phase`, `from` (empty when the TUI saw the run for the first time) |

Keys inside a terminal tab are counted, not logged, and single characters typed into a dialog's input stay
out of the log. `prd` and `phase` are the selected PRD and its phase: the run's, or the status line's without
a run. Writing never fails the TUI; a line that cannot be written is lost. The file has the rights `0600`.

`lazydev activity [--since 7d] [--json]` reads the log: without `--json` a report with sessions, active time
(the gap to the next event, up to five minutes), time and keys per pane and tab and per terminal program, the
keys used most, the actions, the runs the TUI saw with the time until the user did anything after `blocked`,
`error` and `merged`, and a table per day. `--json` prints the events as JSON lines. `--since` takes days
(`14d`), hours (`36h`) or a date (`2026-10-01`).

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

1. **plan:** decides whether the PRD may start next to the open runs of its project (phases waiting, plan,
   setup, implement, review, merge-queue, merge, blocked) or must wait until some of them are merged.
   Without such runs the step ends right away without Claude: `No other open runs in <id>, <prd> starts
   right away`, `after` stays empty. Otherwise Claude gets the built-in prompt `plan` (not part of
   `.lazydev/`, header `# lazydev plan: <prd path>`) with the open runs as `` `<name>` (Phase <phase>):
   `<absolute PRD path>` ``, the worktree's PRD when the run moved it there. It runs in the main folder with
   `--permission-mode default` and only `Read,Glob,Grep`, so it changes nothing. Its answer must end with
   `## Reihenfolge`, a line `Nach: <prd>, …` or `Nach: keine` and `Grund: <sentence>`. Names may carry
   backticks, area or `.md`; a missing section, a missing `Nach:` line or a name that is no open run fails
   the step. PRDs that wait for this one, directly or through others, are dropped with a log line, so the
   order has no cycle. The result goes into `after` and `afterReason` of the run state, the log says
   `Parallel: <reason>` or `After <prds>: <reason>`. Its cost counts like any step.
2. **setup:** `git worktree add -b feat/<prd> <worktree> <base>` (an existing worktree stays, an existing
   branch without worktree is an error). An untracked PRD moves into the worktree and gets committed as
   `docs(prd): add <prd>`. Then the databases (env files, containers, databases, see Databases), then the
   setup hook.
3. **implement:** Claude with the PRD path. Success requires `## Ergebnis` `umgesetzt` and the PRD status
   `umgesetzt (date)` in the worktree. `abgebrochen` with points under `## Offene Fragen` blocks the run (see
   Blocked runs), `abgebrochen` without them fails with `implement aborted`. `## Hinweise` holds remarks
   without a decision and changes nothing.
4. **review:** Claude reviews and commits through the agent `committer`. The worktree must be clean.
5. **merge:** wait for the project's merge slot (phase merge-queue), stop the dev server. If the PRD is the
   only uncommitted file in the worktree, commit it as `docs(prd): resolve blocker of <prd>`. Require a clean
   worktree, `git rebase <base>`; on conflict `rebase --abort` and Claude step merge (`bereit für
   Fast-Forward`, base must be an ancestor, worktree clean; `abgebrochen` with points under `## Offene Punkte`
   blocks the run, without them it fails with `merge aborted`). If the branch was not on the base branch
   before the rebase and `databases` are configured, the databases get created again and the setup hook
   runs again (see Databases). Check hook; if red, Claude step merge with the
   error as context and the check again. Cost and implement model go into the PRD status line, committed as
   `docs(prd): record cost of <prd>`. Wait while the main folder is not on the base branch or has changes
   outside untracked `prd/` files and `.lazydev/` (phase merge-queue, note `main folder: …`, checked every
   5 s). If the base branch moved during the wait, rebase and check again from `git rebase <base>`.
   `git merge --ff-only feat/<prd>` in the main folder, teardown hook, dropping the databases,
   `git worktree remove` and `git branch -d`. Cleanup errors only become a note `cleanup incomplete: …`.

After implement, review and the Claude step merge, nothing under `.lazydev/` may differ from the base branch
in the worktree. A Claude step succeeds only with exit code 0, a last `result` event without error and its
check. Merges always run automatically.

- `s` on an existing run continues with the failed, blocked or next step. Merged, discarded, already running
  or waiting PRDs refuse with a message. A run whose PRD in the worktree still says `blockiert` refuses with
  `<prd> is blocked. Resolve it with f first.`. `s` asks `Start|Continue <prd>? (y/n)`.
- At most 3 runners work at the same time. Above that, the PRD gets phase waiting. A runner that ends starts
  waiting PRDs in start order.
- **Order:** the setup of a PRD starts only when every PRD in `after` is merged or its run state is gone
  (discarded). Until then the PRD has phase waiting and takes no slot: `start` and `startWaiting` skip it,
  and a runner that reaches the setup with an unmerged PRD in `after` saves phase waiting with PID 0 and ends
  without error. The runner that merges the last of them starts it. A PRD in `after` that ends in error,
  blocked or aborted holds the waiting PRD until it is merged or discarded, or until `lazydev after`
  changes the order. `start` and `s` on such a PRD say `<prd> waits for <prds>: <reason>`, a PRD without a
  free slot `3 runners work already. <prd> waits and starts once one ends.`. The run list shows
  `waits for <prds>: <reason>` in place of the last log line, `status` shows `waiting → <prds>`.
- **`lazydev after [<project>] <prd> [<prd>…]`** overrides plan on a run that has not started its setup:
  the PRDs must have runs in the project, none may wait for `<prd>` (`<dep> waits for <prd> already, that
  would be a cycle`), `<prd>` not itself. Without PRDs it clears `after`. `afterReason` becomes
  `set by hand`, an open step plan counts as done, a running plan refuses (`<prd> plans right now, set it
  once plan is done`), a started setup too (`<prd> has started already, phase <phase>`). Then waiting PRDs
  whose turn it is start.
- Ports start at 3500, skipping ports of unmerged runs and ports something listens on (IPv4 and IPv6).
- `x` stops a runner (phase aborted) or aborts a waiting PRD.
- `X` asks, then stops the dev server, runs the teardown hook, drops the databases (also without worktree,
  errors go into the error list), puts a moved PRD back into the main folder with
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
  `claude "<unblock prompt>" --agents '<agents JSON>'` interactively in the worktree. An interactive
  claude takes `--agents` as inline JSON only, not as a file path.
- **Dialog:** the contract makes Claude ask the questions of `## Blocker` numbered with options A to D, wait
  for the answers and work them into the PRD. Then it removes `## Blocker` and sets the status line to
  `geplant` after `Schritt: implement`, to `umgesetzt (<today>)` after `Schritt: merge`. It changes only the
  PRD, commits nothing and ends with `Weiter mit lazydev start <prd> oder s in der TUI.`. The dialog runs
  outside the runner, the next step checks `.lazydev/` anyway.
- **Without the TUI:** `lazydev prompt unblock [--auto] [<project>] <prd>` prints the same prompt with the
  absolute path of the PRD in the worktree as `$ARGUMENTS`, so a Claude Code session in any folder edits
  the right file (see CLI). `--auto` appends the block `## Autonomer Modus`: Claude answers the questions
  itself from the conversation and the code, writes them with the chosen option and a reason into
  `## Entscheidungen`, asks the user only about questions it cannot answer responsibly (data loss, public
  interfaces, running costs, a contradiction to an explicit requirement of the user) and continues the run
  with `lazydev start`.
- **Auto-unblock:** with `autoUnblock` on for the project (see `settings.json`) and fewer than 2 resolve steps
  in the run, the runner records the block as above (status line, `## Blocker`, commit), logs
  `Blocked: <error>, Claude resolves it (auto-unblock)`, sends no notification yet and keeps its PID. Then
  the step `resolve` runs, phase resolve, log `<prd>.resolve.jsonl`, recorded in `steps` like a step but
  skipped when the run picks its next step. Claude gets the built-in prompt `resolve` (not part of
  `.lazydev/`, header `# lazydev resolve: <prd path>`) in the worktree with the permission mode of
  `settings.json` and only `Read`, `Edit`, `Write`, `Glob`, `Grep` and `git status`, `diff`, `log`, `show`.
  It answers the questions like a developer of the project, writes each with the chosen option and a reason
  into `## Entscheidungen`, works them into the PRD, removes `## Blocker` and sets the status line like the
  dialog. Questions about data loss, security, public interfaces, running costs or a contradiction to an
  explicit requirement of the PRD it may not decide when code and PRD give no clear answer; then it changes
  nothing and reports `abgebrochen` with them under `## Offene Fragen`.
  Success needs exit code 0, a last `result` without error, `## Ergebnis` `geklärt`, no new uncommitted
  change outside the PRD (`resolve changed <path>, it may only change the PRD`), nothing changed under
  `.lazydev/`, and a PRD without `blockiert` and without `## Blocker`. Then the runner commits the PRD alone
  as `docs(prd): resolve blocker of <prd>`, logs every decision as `Decision: <point>` and
  `Resolved, <step> runs again`, posts `resolved` and runs the blocked step again in the same runner.
  Otherwise it logs `Not resolved: <error>` (`Claude may not decide: <questions joined with " | ">` for
  `abgebrochen`), restores the PRD with `git checkout HEAD -- <prd>`, and the run ends in phase blocked with the
  error of the block and the `blocked` notification, as without auto-unblock. An abort during resolve
  restores the PRD too and ends in phase aborted. After 2 resolve steps a further block stops the run right
  away. `lazydev auto-unblock [<project>] [on|off]` prints or sets the switch.
- **Continue:** nothing starts by itself, `s` continues with the blocked step. While the PRD in the worktree
  still says `blockiert`, the TUI refuses (see Run) and `start` and a runner end with
  `<prd> is still blocked, resolve it with lazydev prompt unblock <prd> or f in the TUI` without
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
| `blocked` | a step blocks (phase blocked), no `failed` then; with auto-unblock only once resolve did not resolve it | `⊘ Blocked: <area>/<name>` | `0xe67e22` | the blocked step, or resolve |
| `resolved` | resolve answered the questions of a blocked step (auto-unblock) | `↻ Unblocked by Claude: <area>/<name>` | `0x1abc9c` | resolve |
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
  `resolved` adds the field `Decisions` the same way with the points of `## Entscheidungen` of the report.
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

### Databases

`databases` in `config.yaml` gives every worktree its own empty Postgres database in a Docker Compose
service. Only Postgres, no copy of the main database, no seeds. Migrations stay in the setup hook of the
project, which runs after the creation and after every recreation. There are no new hook variables: hooks
and the dev server read the name from the rewritten env file in the worktree.

```yaml
databases:
  - service: db
    env:
      file: .env
      key: DATABASE_URL
  - service: db-test
    compose: docker-compose.dev.yml
    env:
      file: .env.test
      key: NUXT_DATABASE_NAME
```

- **Fields:** `service` (required), `compose` (optional, relative to the main folder), `env.file` (required,
  relative to the main folder) and `env.key` (required). `repo.Read` checks every entry: a missing field is
  the config error `config.yaml: databases[<i>]: <field> missing`, an absolute path or a path with `..` in
  `compose` or `env.file` `config.yaml: databases[<i>]: <field> must be a relative path inside the project`.
  Like broken YAML it shows in the details pane and runs refuse to start.
- **Compose file:** without `compose` the default file `docker compose` finds in the main folder without
  `-f` (`compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`). With `compose` lazydev
  runs `docker compose -f <compose>` in the main folder. lazydev looks for no other files by itself.
- **User:** `POSTGRES_USER` of the `environment` of the service in `docker compose config --format json`,
  `postgres` without it. lazydev needs no password, every command runs through `docker exec` in the
  container.
- **Container:** the running container with the labels `com.docker.compose.project.working_dir` equal to the
  folder of the compose file (the main folder for the default file) and `com.docker.compose.service` equal to
  `service`. If none runs (setup, recreation), lazydev runs `docker compose [-f <compose>] up -d <service>` in
  the main folder and waits up to 60 s until `pg_isready -q -U <user>` succeeds in the container, otherwise
  the error `postgres in service <service> not ready after 60s`.
- **Name:** the base name is the value of `env.key` in `env.file` of the main folder. A value containing
  `://` is a URL, its base name is its path without the leading `/`. Otherwise the whole value is the base
  name. The database of the worktree is `<base name>_<slug>`, above 63 bytes the error
  `database name <name> is longer than 63 bytes`.
- **Env file:** every setup copies each file of `env.file` from the main folder into the worktree and
  replaces the keys of all entries with this file, as `KEY="value"` with `"` escaped as `\"`. In a URL only
  the path changes, scheme, user, password, host, port and query stay. A plain value gets replaced. All other
  lines stay. The file gets mode `0600`. Errors: `<file> missing in the main folder`,
  `<file>: <key> is not set` (missing or empty), `<file>: <key> has no database name` (URL without path), and
  `<file> is not ignored by git, a worktree copy would dirty the worktree` if `git check-ignore -q` fails in
  the worktree. The check runs before writing.
- **Setup:** after the PRD moved and before the setup hook: env files, containers, databases. Without trust
  the setup creates nothing and fails with `databases not created, databases in .lazydev/config.yaml are not
  trusted`. An existing database stays (`Database <name> in <container> exists already`), so `s` goes on
  after an error in the setup hook. Several entries with the same service and name create it once. The run
  state saves the databases as `databases: [{service, name}]`.
- **Recreation:** if the branch is not on the base branch before the rebase of the merge
  (`git merge-base --is-ancestor <base> HEAD` fails), lazydev logs `<base> moved, recreating databases` after
  the rebase and a Claude step merge, drops and creates every database of `databases` again, runs the setup
  hook again, and only then the check hook. This applies to the rebase again after waiting for the main
  folder too. Without `databases` nothing changes.
- **Dropping:** after a merge after the teardown hook and before the worktree goes; an error becomes part of
  `cleanup incomplete: …` and the merge stays successful. On `X` after the teardown hook, also without
  worktree; errors go into the error list of the discard. lazydev drops the databases of the run state, and
  without run state or with an empty list those computed from `config.yaml` and the env files of the main
  folder. What cannot be computed (file or key missing) reports `<file>: database not determinable`. A
  stopped service starts nothing and fails nothing: `Service <service> is not running, database <name>
  stays`.
- **Protection:** lazydev drops only names ending in `_<slug>` with a non-empty slug, otherwise the error
  `database <name> does not belong to the worktree, not dropping it`. It drops with
  `dropdb --if-exists --force`. Names go into SQL only in `SELECT 1 FROM pg_database WHERE datname = '<name>'`
  with doubled `'`, `createdb` and `dropdb` get them as an argument, never through a shell.
- **Step log:** each action gets a runner line: `<file>: <key> → <name>`,
  `Starting service <service>: docker compose … up -d <service>`, `Database <name> in <container> created`,
  `Database <name> in <container> dropped`.

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

### Claude Code skill

lazydev installs a skill `lazydev` for Claude Code, so a normal Claude Code session outside lazydev knows
what lazydev is, how PRDs work and that `lazydev prompt create` prints the create prompt of a project. The
skill replaces no MCP server and no slash command.

- **Place:** `$CLAUDE_CONFIG_DIR/skills/lazydev/`, without the variable `$HOME/.claude/skills/lazydev/`.
  Global for the user, never in a project. The content belongs to the version of the binary.
- **Files:** `SKILL.md` (German like the prompts: frontmatter `name: lazydev` and `description` with the
  triggers, a comment that lazydev manages the file, what lazydev is, the rules for PRDs, a table of
  tasks, under 80 lines) and `references/spec.md`, a byte-equal copy of `SPEC.md`. Nothing else. The copy
  lives in `internal/skill/files/references/spec.md`, `go generate ./internal/skill` refreshes it and a
  test fails when it differs from `SPEC.md`. The files carry no version and no timestamp.
- **Writing:** every embedded file is written only when it is missing or its content differs, atomically
  through a temporary file, mode `0644`, folders `0755`. Files under `skills/lazydev/` that the binary
  does not have get deleted, empty subfolders after them. Nothing outside `skills/lazydev/` changes.
- **When:** the TUI writes the skill at its start in the background, like `lazydev skill install`, and
  `lazydev add` writes it before it ends, so it
  stays current after `lazydev update`, `install.sh` and `make install`. Success and an up-to-date skill
  show nothing. An error sets the footer note `skill install failed: <error>` unless an error or note
  shows already, `lazydev add` prints `lazydev: skill not installed: <error>` on stderr and still ends with 0.
  Runners, `lazydev git` and the other commands write nothing. `"skill": false` in
  `settings.json` switches the automatic write off, `lazydev skill install` works regardless. Removing
  the skill is manual: delete the folder and set `"skill": false`.
- **`lazydev skill install`:** writes the skill and prints `Installed skill lazydev to <dir>.`, or
  `Skill lazydev in <dir> is up to date.` when nothing changed. Exit code 0, errors as `lazydev: <error>`
  with 1.
- **`lazydev prompt create [--auto] [<description>]`:** prints the create prompt of the project on stdout,
  the same one `n` builds. The project folder is the main folder of the repository of the working
  directory (the parent of `git rev-parse --git-common-dir`, so a worktree of a run counts as its project),
  without a git repository the working directory itself. All arguments join with spaces to `$ARGUMENTS`, without
  arguments it is `Die Aufgabe ergibt sich aus dem bisherigen Gespräch mit dem Nutzer.`. `$BASE` is the
  base branch of `config.yaml`, `$BRANCH` the checked out branch. Knowledge files must exist in the
  project folder. The command creates nothing: a folder without `.lazydev/` ends with
  `lazydev: <dir> has no .lazydev/, add the project with lazydev add first` and 1. Errors of `config.yaml`, a
  missing knowledge file or a missing `create.md` end with the error and 1. `--auto` appends the block
  `## Autonomer Modus` after the contract: Claude answers its numbered questions itself, first from the
  conversation, then from code, `CLAUDE.md`, `README.md` and existing PRDs, writes each with the chosen
  option and a reason of one sentence into `## Entscheidungen` of the PRD, and asks the user only questions
  it cannot decide and where a wrong choice is expensive. `prompt unblock` is described under Blocked runs.
  Other steps end with `unknown step: <step>` and the usage, exit code 2.
- lazydev changes no `CLAUDE.md` of a project.

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
| `$XDG_STATE_HOME/lazydev/runs/<id>/<prd>.json` | Run state: projectDir, prd, branch, base, worktree, port, slug, phase, pid, startedAt, steps (name, sessionId, model, exitCode, ok, costUsd, startedAt, endedAt), error, note, after, afterReason, databases (service, name) |
| `…/runs/<id>/<prd>.<step>.jsonl` | Step log (also `resolve`): stream-json with `ts` and runner lines `{"type":"lazydev","time","text"}` |
| `…/runs/<id>/<prd>.runner.log`, `.devpid`, `.dev.log` | Runner output, dev server PID and folder, dev server output |
| `$XDG_STATE_HOME/lazydev/trust/<id>.json`, `usage.json`, `agents.json`, `bin/` | Trust, limits, agents, runner binary |
| `$XDG_STATE_HOME/lazydev/activity.jsonl` | Activity log of the TUI (see Activity log) |
| `$XDG_DATA_HOME/lazydev/worktrees/<id>/<prd>/` | Worktrees |
| `$CLAUDE_CONFIG_DIR/skills/lazydev/` (without the variable `~/.claude/skills/lazydev/`) | Claude Code skill: `SKILL.md`, `references/spec.md` |

State files are written atomically. Merged runs keep their state until discarded.

## CLI

`lazydev` starts the TUI. The TUI is optional: every action on projects, PRDs and runs has a command, so
Claude Code and scripts work without it. `run` and `git` are internal. Errors print `lazydev: <error>` on
stderr and end with 1, wrong arguments print the usage and end with 2. See `lazydev help`.

`<project>` is a project ID or the path of a project in the list. Where the usage shows `[<project>]`, it
is optional: without it, the project of the working directory applies, the main folder of its repository
(parent of `git rev-parse --git-common-dir`), so subfolders and the worktree of a run count. A folder that
is no project in the list ends with `<dir> is no project in the list, add it with lazydev add or name the
project`, an unknown name with `no project "<name>" in the list, add it with lazydev add first`. `<prd>` is
the PRD file name without `.md`.

| Command | Does |
|---|---|
| `add [<path>]` | Adds the folder, without path the main folder of the repository of the working directory, runs `.lazydev/` init (see `.lazydev/`) and writes the skill (see Claude Code skill). Prints `Added <id> (<path>).`, for a folder in the list already `<id> (<path>) is in the list already, .lazydev/ is complete.` No folder: `not a folder: <path>`. |
| `remove <project>` | Removes the project from the list. Folder, run state and worktrees stay, runners keep working (`Its runners keep working until they end.`). |
| `projects [--json]` | ID and path of every project, `--json` as an array of `{id, path}`. |
| `auto-unblock [<project>] [on\|off]` | Prints or sets `autoUnblock` of the project: `Auto-unblock of <id> is on: Claude answers the questions of blocked runs itself, at most 2 times per run.` or `Auto-unblock of <id> is off: blocked runs wait for you.` (see Blocked runs). |
| `trust [<project>]` | Shows hooks, allowed tools and databases of `config.yaml` and saves their hash after `y` (see Trust). |
| `status [<project>] [--all] [--json]` | Without `<project>` outside a project: all projects with a column `PROJECT`. A table of the PRDs sorted by label: phase (`waiting #<n>` with the position in the queue, `→ <prds>` while it waits for PRDs of `after`, with a line `<prd> waits for <prds>: <reason>` below the table), step (the running one, else the one `start` continues with), port (`dev` with a dev server), cost, status value. Below it the error and note of each run and the questions of `## Blocker` of a blocked run. Merged and discarded PRDs only with `--all`, or with an error. `--json` prints an array of objects with `project`, `prd`, `label`, `path`, `file` (absolute, in the worktree while it exists), `status`, `phase`, `queue`, `step`, `branch`, `worktree`, `port`, `cost`, `error`, `note`, `blocker` (`{step, questions}`), `after`, `afterReason`, `waitsFor` (the PRDs of `after` not merged yet), `resolved` (successful resolve steps), `devPid`, `log` (the step log); empty fields are left out. |
| `start [<project>] <prd>` | Starts or continues the runner in the background (see Run). A PRD that waits prints why: `<prd> waits for <prds>: <reason>`, or `3 runners work already. <prd> waits and starts once one ends.` |
| `after [<project>] <prd> [<prd>…]` | Overrides the order plan chose (see Run): `<prd> starts once <prds> are merged.`, without PRDs `<prd> waits for nothing and starts with the next free slot.` The project is optional only when the first argument is no project. |
| `stop [<project>] <prd>` | SIGTERM to the runner, a waiting PRD becomes aborted. |
| `discard [--yes] [<project>] <prd>` | Asks `Delete worktree and branch feat/<prd> of <id>? …`, `--yes` skips the question (see Run, `X`). |
| `wait [<project>] [--json] [--timeout <duration>]` | Projects like `status`. Reads the overview every second and returns with the events of the first read that has any: a run that reaches blocked, error, aborted or merged (`<kind> <id>/<prd>: <error or questions joined with " \| ">`), a run with a new note (`note <id>/<prd>: <note>`) and a run whose resolve step answered the questions of a block (`resolved <id>/<prd>: Claude answered the open questions, see ## Entscheidungen in <file>`). A PRD seen first as merged gives no event. When no run works, sits in the merge queue or waits, it returns right away with `idle: no run works or waits`; after the timeout (Go duration, e.g. `30m`) with `timeout: no event within <duration>`. `--json` prints one object per event: `kind`, `project`, `prd`, `phase`, `from`, `text`, `item` (the object of `status --json`). Always exit code 0. |
| `log [<project>] <prd> [-f]` | The step logs of the run in step order as `<time> <step> <kind> <text>`, the entries of the log tab. `-f` (`--follow`) reads new lines every second until the runner ends, then prints `-- <prd> <phase>`. Without run: `<prd> has no run`. |
| `set-status [<project>] <prd> planned\|merged\|discarded` | The status picker (see Status): same checks, same commit, `Status of <label>: <status>, committed.` A PRD that only its worktree has: `<prd> is in the worktree of its run, its status belongs to the run`. |
| `dev [<project>] <prd> [start\|stop]` | Starts or stops the dev server like `D`, without action it toggles. Start prints PID, port and log. Without run: `<prd> has no worktree yet, start it with lazydev start`. |
| `prompt create [--auto] [<description>]` | See Claude Code skill. |
| `prompt unblock [--auto] [<project>] <prd>` | Prints the unblock prompt of a blocked run (see Blocked runs). Not blocked: `<prd> is not blocked, phase <phase>`. |
| `update` | Installs the newest release over the binary (see Releases). |
| `skill install` | Writes the Claude Code skill (see Claude Code skill). |
| `activity [--since 7d] [--json]` | Reports the activity log (see Activity log). |
| `version`, `--version`, `-v` | Print `lazydev <version>` and end with 0. |
