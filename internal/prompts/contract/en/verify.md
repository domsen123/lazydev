## Rules of lazydev

These rules take precedence over everything above. You work without a human in print mode, questions
are not possible.

- Check the feature of the PRD `$ARGUMENTS` in the running app like a user. Static checks alone are
  not enough.
- Put evidence (screenshots, snapshots, responses, log excerpts) only under `$EVIDENCE`, nowhere else.
  You may overwrite existing files there. Compare the evidence with `## Success criteria` of the PRD.
- The app runs on port `$PORT`, or you start it there with commands that end by themselves.
- If you find an error, fix it in the worktree, run the check commands from `CLAUDE.md` again and
  verify again.
- **Do not commit.** The next step does that. No `git add`, no `git commit`.
- Stop the processes you start yourself before the end.
- You already work in the worktree. No `cd` and no `git -C`. Run every shell command on its own,
  without `&&`, `;`, `||`, pipes and subshells. lazydev refuses compound commands.
- Never start a watch mode or other commands that do not end by themselves.
- Forbidden: `git push`, `git reset --hard`, `git worktree`.
- Change nothing under `.lazydev/`.

### Final report

At the end, write a short report with exactly these sections:

```
## Result
verified | aborted

## Evidence
- <file name under $EVIDENCE>: <what the file shows>

## Changed files
- <path>: <one sentence>, or "none"

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

- `verified` only if the feature works in the app, all checks are `ok`, `## Open questions` says
  "none" and at least one file stands under `## Evidence`.
- Under `## Evidence` name only the file name relative to `$EVIDENCE`, without a path before it. Every
  file named must lie there.
- Otherwise `aborted`. Every open question blocks the run until the user resolves it in the PRD.

lazydev evaluates only `## Result`, `## Evidence` and `## Open questions`. Keep the headings and both
values exactly like this.
