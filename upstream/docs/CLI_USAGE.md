# CLI Usage Guide

This guide covers how to use the `tkn-approvaltask` CLI tool to interact with ApprovalTasks. The CLI provides 4 simple commands to manage approval tasks.

## Table of Contents

- [Available Commands](#available-commands)
- [Command Examples](#command-examples)
- [CLI Reference](#cli-reference)

## Available Commands

The `tkn-approvaltask` CLI provides exactly 4 commands:

1. **`list`** - List all approval tasks
2. **`describe`** - Show detailed information about a specific approval task  
3. **`approve`** - Approve an approval task
4. **`reject`** - Reject an approval task

## Command Examples

### 1. List Approval Tasks

```bash
# List all approval tasks in current namespace
tkn-approvaltask list

# List in specific namespace
tkn-approvaltask list -n production

# List across all namespaces
tkn-approvaltask list -A
```

**Example Output:**
```
NAME                             NumberOfApprovalsRequired   PendingApprovals   Rejected   STATUS
pr-custom-task-beta-8d22w-wait   2                           0                  0          Approved
deployment-approval              3                           1                  0          Pending
security-review                  1                           0                  1          Rejected
```

### 2. Describe Approval Task

```bash
# Show detailed information about an approval task
tkn-approvaltask describe pr-custom-task-beta-8d22w-wait

# Describe in specific namespace
tkn-approvaltask describe deployment-approval -n production
```

**Example Output:**
```
📦 Name:            pr-custom-task-beta-8d22w-wait
🗂  Namespace:       default
🏷️  PipelineRunRef:  pr-custom-task-beta-8d22w

👥 Approvers
   * foo
   * bar
   * user3
   * tekton (Group)
   * example (Group)

👨‍💻 ApproverResponse

Name     ApproverResponse     Message
foo      ✅                    ---
bar      ✅                    ---

🌡️  Status

NumberOfApprovalsRequired     PendingApprovals     STATUS
2                             0                    Approved
```

### 3. Approve an Approval Task

```bash
# Simple approval
tkn-approvaltask approve deployment-approval

# Approval with message
tkn-approvaltask approve deployment-approval -m "Code review completed successfully"

# Approve in specific namespace
tkn-approvaltask approve deployment-approval -n production -m "Security scan passed"
```

**Example:**
```bash
$ tkn-approvaltask approve pr-custom-task-beta-8d22w-wait -m "Tests passed"
ApprovalTask pr-custom-task-beta-8d22w-wait is approved in default namespace
```

### 4. Reject an Approval Task

```bash
# Simple rejection
tkn-approvaltask reject deployment-approval

# Rejection with message
tkn-approvaltask reject deployment-approval -m "Found security vulnerabilities"

# Reject in specific namespace
tkn-approvaltask reject deployment-approval -n production -m "Critical issues found"
```

**Example:**
```bash
$ tkn-approvaltask reject deployment-approval -m "Critical bugs found in testing"
ApprovalTask deployment-approval is rejected in default namespace
```

## CLI Reference

### Global Flags

| Flag | Description | Example |
|------|-------------|---------|
| `-n, --namespace` | Kubernetes namespace | `-n production` |
| `-o, --output` | Output format (json, yaml, wide) | `-o yaml` |
| `--kubeconfig` | Path to kubeconfig file | `--kubeconfig ~/.kube/config` |
| `-v, --verbose` | Verbose output | `-v` |
| `--help` | Show help | `--help` |
