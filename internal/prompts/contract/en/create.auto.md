## Autonomous mode

This section takes precedence over the rules above where they contradict each other.

- Do not ask the user the clarifying questions. Write them numbered with the options A to D and answer
  them yourself: first from the conversation with the user so far, then from the code, `CLAUDE.md`,
  `README.md` and the existing PRDs. With several defensible options, pick the one that fits the
  existing code best.
- Write each question with the chosen option and a reason of one sentence into the section
  `## Decisions` of the PRD. A later run and a blocker dialog need this context.
- Ask the user only when a question can be decided neither from the conversation nor from the code and
  a wrong choice would be expensive, e.g. data loss, public interfaces or running costs. Then ask only
  these questions and wait for the answer.
- The finished PRD still holds no open decisions.
