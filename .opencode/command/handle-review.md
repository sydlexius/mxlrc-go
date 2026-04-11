---
description: "Triage open Copilot/bot PR review comments, fix everything in one pass, reply in batch, push once"
argument-hint: "[PR number -- defaults to current branch's PR]"
tools:
  bash: true
  glob: true
  grep: true
  read: true
  edit: true
  write: true
  task: true
---

# Handle PR Review

Resolve all open Copilot/bot review comments in a single pass. The invariant: **one push,
after all fixes are complete**. Never push per-comment.

**PR number (optional):** "$ARGUMENTS"

---

## Step 1 -- Identify the PR

Resolve `pr_number` and `repo`:

```bash
repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
me=$(gh api user --jq .login)
```

If `$ARGUMENTS` is a number, use it directly. Otherwise detect from the current branch:

```bash
pr_number=$(gh pr view --json number --jq .number)
```

Print the PR URL:

```bash
gh pr view "$pr_number" --json url --jq .url
```

If no PR found, stop: "No open PR found for this branch."

---

## Step 2 -- Fetch all review comments

```bash
# All inline review comments
gh api "repos/$repo/pulls/$pr_number/comments" --paginate \
  --jq '[.[] | select(.body | length > 0) | {id, user: .user.login, path, line, body}]'

# All review-level comments (review summaries)
gh api "repos/$repo/pulls/$pr_number/reviews" --paginate \
  --jq '[.[] | select(.body | length > 0) | {id, user: .user.login, state, body}]'
```

---

## Step 3 -- Identify open (unreplied) comments

A comment is **open** if:
1. It is a top-level comment from a reviewer bot (login contains `copilot` or ends with
   `[bot]`, case-insensitive) or from a human reviewer
2. AND there is no subsequent reply in the same thread from `$me`

Print a numbered list of open comments:
```
Open review comments (N total):
1. [id: 123456] path/to/file.go -- "First line of comment body..."
2. [id: 789012] cmd/mxlrcsvc-go/main.go -- "First line..."
```

If there are no open comments, say: "No open review comments. Nothing to do." and stop.

---

## Step 4 -- Read and categorize each comment

For each open comment, read the full body and the referenced file/line. Assign one of:

| Category | Meaning |
|----------|---------|
| `bug` | Real code defect — must fix |
| `test-gap` | Missing test coverage for a real gap — should fix |
| `false-positive` | Established pattern, known behavior, or intentional design |
| `already-fixed` | Corrected in a later commit; reply needed but no code change |
| `wont-fix` | Valid suggestion but out of scope for this PR |

### Propagation sweep

Before printing the triage table, check whether any bug/pattern recurs elsewhere:

```bash
grep -rn "problematic_pattern" --include='*.go' .
```

Add every additional occurrence to the fix scope and note them in the Summary column.

Print the full triage table before making any changes:

```
## Triage

| # | ID     | Category       | File              | Summary |
|---|--------|----------------|-------------------|---------|
| 1 | 123456 | bug            | internal/app/app.go | error path drops context |
| 2 | 789012 | false-positive | cmd/mxlrcsvc-go/main.go | godotenv load pattern |
```

Ask: "Does this triage look right? (yes / adjust N to <category>)"

Wait for confirmation before proceeding.

---

## Step 5 -- Implement all fixes

For every `bug` or `test-gap` comment:
- Read the relevant code
- Make the minimal correct fix
- Do NOT push yet

After all edits, run tests:

```bash
go test -count=1 -race ./... 2>&1
```

If tests fail: stop. Fix failures before continuing.

Also run the linter:

```bash
golangci-lint run ./... 2>&1
```

If lint errors: fix them before continuing.

---

## Step 5.5 -- Local review of fixes

Run a focused review of the changed files before committing:

```bash
git diff --name-only
```

Launch the `gsd-code-reviewer` agent against the changed files. If it flags critical
issues, fix them before committing. For important (non-blocking) findings, present them
and ask: "Fix now or proceed? (fix/proceed)"

---

## Step 6 -- Compose replies

Draft a reply for each open comment:

**bug / test-gap (fixed):**
```
Fixed in <sha>. <one-sentence description of what changed>.
```
(Fill in the real sha after committing in Step 7.)

**false-positive:**
```
<Brief explanation of why this is correct — one or two sentences.>
```

**already-fixed:**
```
Fixed in <earlier-sha>.
```

**wont-fix:**
```
Acknowledged — out of scope for this PR. Tracking separately as #<issue> or leaving for follow-up.
```

---

## Step 7 -- Commit, get SHA, post replies

Commit all fixes:

```bash
git add -p
git commit -m "fix: address PR review findings

<bullet list of what was fixed>"
```

Get the short SHA:
```bash
git rev-parse --short HEAD
```

Substitute the real SHA into all "Fixed in <sha>" drafts.

Post all replies in one batch using the GitHub API:

```bash
# Reply to an inline comment
gh api "repos/$repo/pulls/$pr_number/comments/$comment_id/replies" \
  -f body='<reply text>'

# Reply to a review-level comment as a PR comment
gh pr comment "$pr_number" --body '<reply text>'
```

Log each reply as it completes.

---

## Step 8 -- Push

```bash
git push origin $(git branch --show-current) 2>&1
```

Report the result. If the push fails, explain why — do not retry automatically.

---

## Step 9 -- Summary

Print:

```
## Done -- PR #$pr_number

- Fixed: $fixed_count (bug/test-gap)
- Dismissed: $dismissed_count (false-positive/wont-fix)
- Noted: $noted_count (already-fixed)
- Replied: $total_count total
- Pushed: $sha to $branch
```

Assess whether a Copilot re-review is warranted:

**Recommend re-review** when fixes touched:
- Error handling paths
- Concurrency or shared state
- Security-sensitive code (token handling, file paths)
- Substantial new code

**Skip re-review** when fixes were:
- Comment/doc-only changes
- Trivial one-line corrections
- Test-only changes
- Style/formatting fixes

Print the recommendation. Note: Copilot re-review must be triggered manually from
the GitHub PR page — the API does not support re-requesting review from bot accounts.
