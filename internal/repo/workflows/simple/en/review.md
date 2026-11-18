# Review and commit

## 1. Structural review

Start the agent `structural-completeness-reviewer`. It checks the changes of this branch against
`$BASE` (`git diff $BASE...HEAD`) and all uncommitted changes:

- The changes are fully integrated.
- Old code is removed completely.
- No technical debt arises.
- The structure stays intact.

## 2. Fix the findings

Fix the findings of the agent right away. Do not change any behavior the PRD defines. Change files only
with the tools Edit and Write. If you changed something, run the check commands from `CLAUDE.md` again
and fix errors.

## 3. Commit

Start the agent `committer`. It commits all changes in the worktree.
