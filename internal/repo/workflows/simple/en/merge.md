# Bring the branch onto the target branch

## Your task

Bring the branch from `$ARGUMENTS` onto the target branch `$BASE`. An error context may follow the
branch name, e.g. a check that failed after the rebase.

## Procedure

1. **Understand the intent.** Read the PRD of the branch under `prd/` (file name like the branch
   without `feat/`) and `git log $BASE..HEAD`.
2. **Rebase.** If no rebase is running, start `git rebase $BASE`. Resolve every conflict so both sides
   stay: the changes from `$BASE` and the intent of the branch. After each resolved conflict run
   `git add` and `git rebase --continue`.
3. **Generated files.** Treat generated files like migrations by the rules of `CLAUDE.md`. Never merge
   them by hand.
4. **Check.** Run the check commands from `CLAUDE.md` and fix errors. If the error context names a
   check, fix that error first. Change files only with Edit and Write.
5. **Commit.** The agent `committer` commits corrections after the rebase.
