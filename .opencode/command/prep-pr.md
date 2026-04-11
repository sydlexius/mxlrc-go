---
description: "Run all pre-push checks then push — the full pre-PR gate"
argument-hint: "[optional: short description of what this PR does]"
tools:
  bash: true
  glob: true
  grep: true
  read: true
  task: true
---

# PR Preparation Gate

Run every pre-push check in order. Gate on failures. Push only when clean.

**Optional context:** "$ARGUMENTS"

---

## Step 1 -- Orient

```bash
base=$(git merge-base main HEAD)
git branch --show-current
git log main..HEAD --oneline
git diff "$base"..HEAD --stat
```

Report:
- Current branch name
- Number of commits ahead of main
- Files changed (summary)

If on `main`, stop immediately: "You are on main. Create a feature branch first."

---

## Step 2 -- Tests

```bash
go test -count=1 -race ./... 2>&1
```

If any test fails: print the failures, stop, and say:
"Fix failing tests before proceeding. Do not push broken code."

If tests pass: note it and continue.

---

## Step 3 -- Build

```bash
go build ./... 2>&1
```

If the build fails: print the errors, stop, and say:
"Fix build errors before proceeding."

---

## Step 4 -- Lint

```bash
golangci-lint run ./... 2>&1
```

If golangci-lint is not installed, skip with a warning.

If any linter errors: print them, stop, and say:
"Fix lint errors before proceeding."

---

## Step 5 -- Vulnerability check

```bash
govulncheck ./... 2>&1
```

If govulncheck is not installed, skip with a warning.

If any vulnerabilities found: present them and ask:
"Vulnerabilities found. Fix them now, or proceed anyway? (fix/proceed)"

Wait for the user's answer before continuing.

---

## Step 6 -- Rename completeness check

If the diff contains any renamed functions, variables, types, or constants:

```bash
base=$(git merge-base main HEAD)
git diff "$base"..HEAD | grep '^-.*func \|^-.*type \|^-.*var \|^-.*const ' | \
  sed 's/^-//' | grep -oE '[A-Z][a-zA-Z0-9]+'
```

For each old name found, grep the full codebase for remaining references:

```bash
grep -rn "OldName" --include='*.go' .
```

**Flag as CRITICAL** if the old name still appears in code (excluding the diff's `-`
lines and comments). Incomplete renames cause compilation errors or silent behavior changes.

---

## Step 7 -- Local code review

Run a focused review of the changed files. Launch the `gsd-code-reviewer` agent
against the diff:

```bash
base=$(git merge-base main HEAD)
git diff "$base"..HEAD --name-only
```

If any Critical findings: stop. Say "Fix all critical issues before pushing."

If Important findings: present them and ask:
"There are important (non-blocking) findings. Fix them now, or proceed anyway? (fix/proceed)"

Wait for the user's answer before continuing.

---

## Step 8 -- Push

```bash
git push origin $(git branch --show-current) 2>&1
```

If the branch has no upstream yet, use `-u`:

```bash
git push -u origin $(git branch --show-current) 2>&1
```

Report the push result. If it fails (non-fast-forward, auth error, etc.), stop and
explain — do not retry automatically.

---

## Step 9 -- PR creation offer

After a successful push, check if a PR already exists:

```bash
gh pr view 2>&1
```

If no PR exists, offer to create one:
"Push succeeded. Create the PR now? (yes/no)"

If yes, determine which issue(s) this branch closes:

1. Parse the branch name for an issue number (e.g. `fix/123-some-desc` or `feat/456-thing`).
2. Scan commit messages for `#N` references.
3. Check `$ARGUMENTS` for explicit issue numbers.

Collect all discovered issue numbers. Then create the PR:

```bash
gh pr create --title "<branch-description>" --body "$(cat <<'EOF'
## Summary
<bullet points from $ARGUMENTS or inferred from commit message>

Closes #N

## Test plan
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run ./...` passes
- [ ] Manual smoke test: binary runs, `--help` works, no-token error is clean
EOF
)"
```

Include one `Closes #N` line per issue. If no issue number can be determined, ask:
"Which issue(s) does this PR close? (e.g. #8)"

If a PR already exists, print its URL and say "PR already open."

**Note:** The first push is the only chance for a clean Copilot review pass. Steps 2–7
gate aggressively so issues are caught before the PR is opened, not after.
