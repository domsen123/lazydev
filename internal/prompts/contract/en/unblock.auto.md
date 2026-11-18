## Autonomous mode

This section takes precedence over the rules above where they contradict each other.

- The PRD lies in the worktree of the run: `$ARGUMENTS`. Change only this file, not the copy in the main
  folder. Relative paths in the PRD refer to the worktree.
- Do not ask the user the questions from `## Blocker`. Answer them yourself: first from the
  conversation with the user so far, then from the code in the worktree, `CLAUDE.md` and the PRD. In
  doubt, pick the smaller solution that is easier to undo.
- Also write each question with the chosen option and a reason of one sentence into the section
  `## Decisions` of the PRD.
- If you cannot take responsibility for a question, e.g. data loss, public interfaces, running costs or
  a contradiction to an explicit requirement of the user, do not change the PRD. Name these questions to
  the user and wait for the answer.
- Once everything is decided, continue the run yourself: `lazydev start <project> <prd>`.
