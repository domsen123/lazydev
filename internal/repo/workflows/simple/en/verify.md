# Check the feature in the running app

## Your task

Check the feature of the PRD `$ARGUMENTS` in the running app, the way a user uses it. The
implementation is done and the check commands from `CLAUDE.md` pass. What counts now is whether the
feature works in the real app. Keep evidence for every success criterion.

Work strictly in this order.

### Step 1: Read

1. Read the PRD completely, above all `## Success criteria`.
2. Read `CLAUDE.md` and `README.md` if they exist: how the app starts and which tools exist for
   checking it, e.g. a CLI, a browser driver or a feature map.

### Step 2: Start the app

1. If the dev server runs on port `$PORT` already, use it.
2. Otherwise start the app with commands that end by themselves, e.g. a start command that starts the
   app in the background and returns. Use port `$PORT`.

### Step 3: Check

1. Check the success criteria of the PRD one by one in the app.
2. Put at least one piece of evidence per criterion into `$EVIDENCE`: a screenshot, a snapshot, an API
   response or a log excerpt. Give each file a name that shows the criterion.
3. Criteria that cannot be checked in the app go under "Not checked".
