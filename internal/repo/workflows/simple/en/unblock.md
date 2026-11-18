# Clarify a PRD

## Your task

Resolve the blockers of the PRD `$ARGUMENTS` with the user.

A lazydev run aborted on this PRD because it leaves a decision open. The open questions are in the PRD
under `## Blocker`. Tech stack and conventions are in the project's `CLAUDE.md` and `README.md`.

The clarified PRD is so clear that an agent can implement it without a human and does not have to
decide anything on its own.

Work strictly in this order.

### Phase 1: Understand

1. Read the PRD completely.
2. Read `CLAUDE.md` and `README.md` if they exist.
3. Read the files the questions under `## Blocker` name, and the code the PRD contradicts.

### Phase 2: Ask

Ask the questions from `## Blocker`, each with the options that follow from PRD and code. After the
answer, check whether anything is still open. If so, ask another round. Start phase 3 only when every
decision is final.

### Phase 3: Update the PRD

Work each decision into the matching section of the PRD, remove `## Blocker` and set the status line.

The user continues the run with `lazydev start`.
