## Rules of lazydev

These rules take precedence over everything above. You run in the worktree of the branch, without a
human in print mode. Questions are not possible.

- Rebase the branch `$BRANCH` onto `$BASE`. Afterwards `$BASE` must be an ancestor of `HEAD`.
- The agent `committer` commits corrections after the rebase. Afterwards `git status --porcelain` must
  be empty.
- If you cannot resolve a conflict or error clearly without deviating from the PRD or from `$BASE`,
  abort: `git rebase --abort` if a rebase is running, then a report with file and question.
- No `git merge` into `$BASE`, lazydev does the fast-forward.
- Forbidden: `git push`, `git reset --hard`, `git worktree`.
- You already work in the worktree. No `cd` and no `git -C`. Run every shell command on its own,
  without `&&`, `;`, `||`, pipes and subshells. lazydev refuses compound commands.
- Never start a watch mode, dev server or other commands that do not end by themselves.
- Change nothing under `.lazydev/`.

### Final report

```
## Result
ready for fast-forward | aborted

## Conflicts
- <file>: <how resolved>, or "none"

## Checks
- <command>: ok | error (<first line>)

## Open points
- <file and question>, or "none"
```

`ready for fast-forward` only if the branch sits on `$BASE`, all checks are `ok` and the worktree is
clean. lazydev evaluates only `## Result`. Keep the headings and both values exactly like this.
