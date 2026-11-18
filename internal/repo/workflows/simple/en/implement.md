# Implement a PRD

## Your task

Implement the PRD `$ARGUMENTS` completely. Tech stack and conventions are in the project's `CLAUDE.md`
and `README.md`. Whatever you may not decide goes into the final report.

> **Ground rule: no decisions of your own.** The PRD holds final decisions only. If it leaves a point
> open that you need for the implementation, you do not decide it yourself. Abort and name the
> question in the final report under "Open questions". Small details that existing code clearly
> dictates are no open decisions.

Work strictly in this order.

### Step 1: Read

1. Read the PRD completely. If the file is missing, abort.
2. Read `CLAUDE.md` and `README.md` if they exist.
3. Read the check commands from `CLAUDE.md`, e.g. `make test` or `go test ./...`.
4. Read the files the PRD names under `## Implementation` and `## Technical notes` before you change
   them.

### Step 2: Implement

1. Implement the steps of `## Implementation` in their order.
2. Follow `## Technical notes` and the conventions of `CLAUDE.md`.
3. Change files only with the tools Edit and Write. Scripts like `python3`, `sed -i` or heredocs are
   not allowed.
4. Write the tests the PRD or `CLAUDE.md` ask for.
5. Update the docs the PRD names.

### Step 3: Check

1. Run all check commands of the project and fix every error. Repeat until all pass.
2. If Claude Code refuses a check command because it is not allowed, abort. Name the command under
   "Open questions".
3. Compare the result with `## Success criteria`.
4. If an error cannot be fixed without deviating from the PRD, abort and name it under
   "Open questions".
5. Criteria that can only be checked by hand go under "Not checked". They do not block the
   implementation.
