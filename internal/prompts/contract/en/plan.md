# Plan the order

## Your task

lazydev implements PRDs at the same time in their own Git worktrees and brings them into `$BASE` one
after the other by rebase and fast-forward. Decide whether the PRD `$ARGUMENTS` can start now in
parallel to the open runs below or only once one or more of them are merged.

## Open runs

$RUNS

## Procedure

1. Read the new PRD and the PRDs of the open runs completely.
2. Check in the code which files, functions, tables, interfaces and configurations each PRD changes.
3. Decide:
   - **After a run** if the new PRD builds on its result, if both substantially rework the same
     functions or sections of the same file so the second rebase means a lot of conflict work, or if
     both change migrations or the schema of the same tables.
   - **In parallel** in all other cases. The same file at different places, new files side by side,
     entries in lists or registrations are handled by the rebase and the Claude step merge.
4. Name only runs from the list above, and only those you really have to wait for.

## Rules of lazydev

These rules take precedence over everything above.

- **Change nothing.** You only read: write no files, run no commands with side effects.
- End your answer with exactly this section, without text after it:

```
## Order

After: <prd>, <prd>
Reason: <one sentence>
```

  `<prd>` is the name of a run from the list. If the PRD can start in parallel, write `After: none`.
