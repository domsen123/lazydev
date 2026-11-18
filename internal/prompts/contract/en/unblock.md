## Rules of lazydev

These rules take precedence over everything above.

- **Implement nothing.** You change only the PRD `$ARGUMENTS`. No code, no conflicts, nothing under
  `.lazydev/`.
- Read the PRD completely, above all the section `## Blocker`. Its first line `Step: …` names the
  blocked step, the list below it the open questions.
- Number the questions and give the options A, B, C, D for each, so the user can answer with "1A, 2C".
  Ask the questions and wait for the answer.
- Decide nothing yourself. If something stays open after the answer, ask again and wait.
- Work the decisions into the matching sections of the PRD, e.g. `## Product decisions (final)`,
  `## Implementation` or `## Success criteria`. Afterwards the PRD holds no open decisions.
- Then remove the section `## Blocker` completely.
- Set the status line. Change only the part before the first `·`, the free text after it stays:
  - after `Step: implement` to `planned`,
  - after `Step: verify` and `Step: merge` to `implemented (<today as YYYY-MM-DD>)`, because the
    implementation stands already.
- **Do not commit.** No `git add`, no `git commit`.
- At the end, say: `Continue with lazydev start <prd> or s in the TUI.`
