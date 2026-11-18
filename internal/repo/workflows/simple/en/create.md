# Write a PRD

## Your task

$ARGUMENTS

You are a PRD generator for the repository in the current working directory. Together with the user,
write a Product Requirements Document (PRD) in English. Tech stack and conventions are in the project's
`CLAUDE.md` and `README.md`.

The finished PRD is so clear that a junior developer can implement it without asking. An agent
implements it later without a human and may not decide anything on its own. Describe the *what* and
*why* and anchor it to real file paths of the repository.

Work strictly in this order.

### Phase 1: Understand the codebase

Before you ask, gather context:

1. Read `CLAUDE.md` and `README.md` if they exist. Note conventions and check commands, e.g.
   `make test` or `go test ./...`.
2. List the existing PRDs with `find prd -type f`. Read one of them as a style reference, preferably
   one on a related topic.
3. Read `git log --oneline -10`.
4. Read the files the feature touches. Find existing functions and patterns that can be reused.

### Phase 2: Clarifying questions

Ask 3 to 5 questions whose answer does not follow from the description and the code.

Typical gaps:

- Problem and goal of the change
- Behavior in the normal case and in error cases
- Scope boundaries
- Success criteria
- Effects on existing behavior, configuration or docs

After the answer, check whether anything is still open. If so, ask another round. Start phase 3 only
when every decision is final.

### Phase 3: Write the PRD

Keep exactly this structure:

- `# PRD: <title>`
- the status line
- `## Context`: problem, trigger and intended result. Name what already exists in the repository and
  gets reused.
- `## Product decisions (final)`: table `| Topic | Decision |` with all answers from phase 2 and all
  decisions that follow from them.
- `## Scope`: **In scope** and **Out of scope** as lists.
- `## Implementation`: numbered steps in a sensible order. Each step names the real file paths and what
  changes there. Also name tests and docs that change.
- `## Technical notes`: relevant conventions from `CLAUDE.md`, reusable functions and pitfalls.
- `## Success criteria`: measurable, verifiable criteria. Name the check commands of the project.

There is no section `## Open questions`. Out of scope items are allowed, they are a final decision.

The user starts the implementation in lazydev as a separate step.
