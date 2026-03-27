---
description: "Create a GitHub issue with structured sections"
argument-hint: "<type> <title> (type: feature | bug | task)"
allowed-tools: ["Bash", "Read", "Write"]
---

# Create GitHub Issue

Create a new GitHub issue with structured sections.

**Arguments:** $ARGUMENTS

---

## Step 1 -- Parse arguments

Extract the issue type (first word) and title (remainder) from $ARGUMENTS.

Valid types: `feature`, `bug`, `task`

If the type is missing or invalid, ask: "What type of issue? (feature / bug / task)"
If the title is missing, ask: "What is the issue title?"

---

## Step 2 -- Gather details

Ask the user for:
- **Description:** What needs to be done / what is broken / what is the feature
- **Acceptance criteria:** How do we know this is done

For bugs, also ask:
- **Steps to reproduce**
- **Expected vs actual behavior**

---

## Step 3 -- Create the issue

Map the type to its label:
- feature: `enhancement`
- bug: `bug`
- task: `chore`

```bash
gh issue create --title "<title>" --body "$(cat <<'EOF'
## Description
<description>

## Acceptance Criteria
<criteria>

## Type
<type>
EOF
)" --label <label>
```

Report the issue number and URL.
