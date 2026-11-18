# Resolve a blocker yourself

## Your task

A lazydev run aborted on the PRD `$ARGUMENTS` because it leaves a decision open. The open questions are
in the PRD under `## Blocker`. You work without the user: decide the questions yourself, the way an
experienced developer of this project would decide them, and work the decisions into the PRD. Then
lazydev continues the run.

Work in this order.

1. Read the PRD completely, above all `## Blocker`. Its first line `Step: …` names the blocked step,
   the list below it the open questions.
2. Read `CLAUDE.md` and `README.md` if they exist, and the code the questions concern. The worktree
   also holds the uncommitted changes of the aborted step, `git status` and `git diff` show them.
3. Answer every question. Pick the option that fits the existing code, the conventions and the goal of
   the PRD best, in doubt the smaller one that is easier to undo.

## Rules of lazydev

These rules take precedence over everything above.

- **Change only the PRD `$ARGUMENTS`.** No code, nothing under `.lazydev/`. Do not commit.
- **Do not decide** when a question concerns data loss, security, public interfaces, running costs or
  a contradiction to an explicit requirement of the PRD and code and PRD give no clear answer. Then do
  not change the PRD at all, not for the other questions either, and report `aborted`.
- When you decide:
  - Work each decision into the matching section of the PRD, e.g. `## Implementation` or
    `## Success criteria`.
  - Write each question with the chosen option and a reason of one sentence into the section
    `## Decisions` of the PRD. If it is missing, add it.
  - Remove the section `## Blocker` completely.
  - Set the status line, only the part before the first `·`: after `Step: implement` to `planned`,
    after `Step: verify` and `Step: merge` to `implemented (<today as YYYY-MM-DD>)`, because the
    implementation stands already.
- End your answer with exactly this report:

```
## Result
resolved

## Decisions
- <question>: <chosen option>, <reason>

## Open questions
- none
```

  With `aborted`, `## Result` says `aborted` and `## Open questions` lists every question you may not
  decide, with the reason.
