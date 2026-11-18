## Rules of lazydev

These rules take precedence over everything above. You work without a human in print mode, questions
are not possible.

- Check with `git branch --show-current` that the branch is `$BRANCH`. Do not switch branches. If it is
  another one, abort.
- **Do not commit.** The next step does that. No `git add`, no `git commit`. Exception: `git mv` when
  the PRD moves a file.
- Set the status line of the PRD `$ARGUMENTS` from `planned` to `implemented (<today as YYYY-MM-DD>)`,
  only if the implementation and all checks were completely successful. Change only the part before
  the first `·`, the free text after it stays:

  ```
  > Status: implemented (2026-10-20) · Scope: …
  ```
- You already work in the worktree. No `cd` and no `git -C`. Run every shell command on its own,
  without `&&`, `;`, `||`, pipes and subshells. lazydev refuses compound commands.
- Never start a watch mode, dev server or other commands that do not end by themselves.
- Forbidden: `git push`, `git reset --hard`, `git worktree`.
- Change nothing under `.lazydev/`.

### Final report

At the end, write a short report with exactly these sections:

```
## Result
implemented | aborted

## Changed files
- <path>: <one sentence>

## Checks
- <command>: ok | error (<first line>)

## Open questions
- <decision the user has to make, with file and place>, or "none"

## Notes
- <remark without a decision>, or "none"

## Not checked
- <success criterion and reason>, or "nothing"
```

Rules for `## Result`:

- `implemented` only if all checks are `ok` and `## Open questions` says "none".
- Otherwise `aborted`. The status line then stays at `planned`.
- `## Open questions` only for decisions the user has to make. Every open question blocks the run
  until the user resolves it in the PRD.
- Remarks without a decision, e.g. a `cd` used by mistake, go under `## Notes`.
- `## Notes` and `## Not checked` do not affect the result.

lazydev evaluates only `## Result` and `## Open questions`. Keep the headings and both values exactly
like this.
