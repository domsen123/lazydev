# lazydev-runs

A Claude Code mod that shows the PRD runs of [lazydev](https://github.com/domsen123/lazydev) in a pane
beside the chat, like the agents panel: cost, running and waiting runs at the top, then every running PRD
with its phase, model, port, the steps done (`3/5 · implement`) as a progress bar, cost and time, the
blocked runs with their open questions, the queue with what each PRD waits for, and the planned PRDs.

It reads `lazydev status --here --json` in the folder of the session (only its project) every 3 s while the pane is open. The pane
opens by itself once a run works, waits or is blocked; `/lazydev-runs` shows or hides it.

lazydev installs it with `lazydev mod install` into `~/.claude/skills/lazydev-runs/` and keeps it current
on every start of the TUI. `lazydev mod uninstall` removes it. It needs `lazydev` in the `PATH`.
