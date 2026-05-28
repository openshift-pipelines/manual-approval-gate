---
name: commit-message
description: >-
  Create properly formatted conventional commit messages for
  manual-approval-gate. Use when asked to "create a commit", "generate
  commit message", "commit changes", or "make a commit". Applies component
  scopes, line length validation, DCO Signed-off-by, and Assisted-by trailers.
license: Apache-2.0
metadata:
  project: manual-approval-gate
  allowed-tools: Read Grep Glob Bash(git diff:*) Bash(git log:*) Bash(git status)
---

# Conventional Commit Message Creation

Create properly formatted conventional commit messages following Tekton
community standards with line length validation and required trailers.

## Format

```text
<type>(<scope>): <description>

[optional body]

Signed-off-by: <name> <email>
Assisted-by: <model> (via <tool>)
```

## Workflow

1. **Analyze changes**: Run `git status` and `git diff --staged` to understand modifications
2. **Determine type and scope**: See tables below
3. **Generate message**: Subject line ≤72 chars, body wrapped at 72 chars
4. **Add trailers**: Signed-off-by (from git config) and Assisted-by
5. **Confirm with user**: Display message and wait for approval before committing

**CRITICAL**: Never commit without explicit user confirmation.

## Type Selection

| Type | When to use |
|------|-------------|
| `feat` | New features or capabilities |
| `fix` | Bug fixes |
| `docs` | Documentation only |
| `refactor` | Code restructuring without behavior change |
| `test` | Adding or updating tests |
| `chore` | Maintenance (deps, CI, build) |
| `build` | Build system or tooling changes (go version, Makefile) |
| `ci` | Changes to .github/workflows/ or .tekton/ directory |

## Scope Selection

Use the affected component as scope:

| Scope | When |
|-------|------|
| `controller` | Changes to `pkg/reconciler/approvaltask/` or `cmd/controller/` |
| `webhook` | Changes to `pkg/reconciler/webhook/` or `cmd/webhook/` |
| `cli` | Changes to `pkg/cli/` or `cmd/tkn-approvaltask/` |
| `api` | Changes to `pkg/apis/` |
| `config` | Changes to `config/kubernetes/` or `config/openshift/` |
| `e2e` | Changes to `test/` |
| `deps` | Dependency updates |

## Line Length Rules

- **Subject**: ≤72 characters (includes `type(scope): `)
- **Body**: wrap at 72 characters per line
- **Blank line** required between subject and body

## Required Trailers

### Signed-off-by

Detect from git config:

```bash
git config user.name
git config user.email
```

Format: `Signed-off-by: Name <email>`

### Assisted-by

Always include when AI assisted: `Assisted-by: <model> (via <tool>)`

## Commit Execution

Use heredoc for proper multi-line handling:

```bash
git commit -m "$(cat <<'EOF'
type(scope): subject line

Body text wrapped at 72 characters.

Signed-off-by: Name <email>
Assisted-by: Model (via Tool)
EOF
)"
```
