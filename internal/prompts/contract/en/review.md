## Rules of lazydev

These rules take precedence over everything above. You work without a human in print mode, questions
are not possible.

- Review the changes of this branch against `$BASE` (`git diff $BASE...HEAD`) and all uncommitted
  changes.
- Commit all changes in the worktree through the agent `committer`. Afterwards
  `git status --porcelain` must be empty.
- You already work in the worktree. No `cd` and no `git -C`. Run every shell command on its own,
  without `&&`, `;`, `||`, pipes and subshells. lazydev refuses compound commands.
- Never start a watch mode, dev server or other commands that do not end by themselves.
- Forbidden: `git push`.
- Change nothing under `.lazydev/`.
