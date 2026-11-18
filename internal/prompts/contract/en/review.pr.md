### Pull request

This project merges through a pull request. After the commit, write title and description of the pull
request to `$EVIDENCE/pr.md`:

- First line `# <title>`: a Conventional Commit in English unless `CLAUDE.md` says otherwise, e.g.
  `# feat(auth): add bearer tokens for mobile clients`.
- Then the description after `.github/pull_request_template.md` of the repository if the file exists,
  otherwise with the sections `## Summary`, `## Evidence` and `## PRD`.
- Under Evidence go the files from `$EVIDENCE/report.md` with their names and the path of the folder
  `$EVIDENCE`. Without `report.md` it says `No runtime verification ran (.lazydev/verify.md is missing).`
- Under PRD goes the path of the PRD.
- The file lies outside the worktree and is not committed. Upload no images, lazydev takes the
  description as text only.
