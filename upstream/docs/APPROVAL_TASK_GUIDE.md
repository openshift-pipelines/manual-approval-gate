# ApprovalTask Guide

This guide provides comprehensive documentation for using ApprovalTask in your Tekton Pipelines to add manual approval points.

## Table of Contents

- [Overview](#overview)
- [ApprovalTask Structure](#approvaltask-structure)
- [Basic Examples](#basic-examples)
- [Advanced Examples](#advanced-examples)
- [Status Fields](#status-fields)

## Overview

ApprovalTask is a Kubernetes Custom Resource that allows you to add manual approval gates in your CI/CD pipelines. When a pipeline reaches an ApprovalTask, it pauses execution until the required number of approvals are received from designated approvers.

## ApprovalTask Structure

### Complete ApprovalTask Example

```yaml
apiVersion: openshift-pipelines.org/v1alpha1
kind: ApprovalTask
metadata:
  name: deployment-approval
  namespace: my-project
spec:
  approvers:
  - name: alice
    type: User
    input: pending
    message: ""
  - name: bob
    type: User
    input: pending
    message: ""
  - name: dev-team
    type: Group
    input: approve
    message: ""
    users:
    - name: charlie
      input: approve
    - name: diana
      input: approve
  numberOfApprovalsRequired: 2
  description: "Approve deployment to production environment"
status:
  state: pending
  approvers:
  - alice
  - bob
  - dev-team
  approvalsRequired: 2
  approvalsReceived: 0
  approversResponse: []
  startTime: "2024-01-15T10:30:00Z"
```

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `approvers` | []ApproverDetails | Yes | List of users/groups who can approve |
| `numberOfApprovalsRequired` | int | Yes | Number of approvals needed |
| `description` | string | No | Description of what needs approval |

### ApproverDetails Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Username or group name |
| `type` | string | Yes | "User" or "Group" |
| `input` | string | Yes | Current state: "pending", "approve", "reject" |
| `message` | string | No | Message from approver |
| `users` | []UserDetails | No | Group members (for Group type) |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `state` | string | Overall state: "pending", "approved", "rejected" |
| `approvers` | []string | List of approver names |
| `approvalsRequired` | int | Number of approvals required |
| `approvalsReceived` | int | Number of approvals received so far |
| `approversResponse` | []ApproverState | Detailed response from each approver |
| `startTime` | *metav1.Time | When the approval task started |

## Basic Examples

### 1. Simple User Approval

```yaml
apiVersion: openshift-pipelines.org/v1alpha1
kind: ApprovalTask
metadata:
  name: simple-approval
  namespace: default
spec:
  approvers:
  - name: alice
    type: User
    input: pending
  numberOfApprovalsRequired: 1
  description: "Simple approval for feature deployment"
```

### 2. Multiple User Approval

```yaml
apiVersion: openshift-pipelines.org/v1alpha1
kind: ApprovalTask
metadata:
  name: multi-user-approval
  namespace: default
spec:
  approvers:
  - name: alice
    type: User
    input: pending
  - name: bob
    type: User
    input: pending
  - name: charlie
    type: User
    input: pending
  numberOfApprovalsRequired: 2
  description: "Requires 2 out of 3 approvals for production deployment"
```

### 3. Group-Based Approval

```yaml
apiVersion: openshift-pipelines.org/v1alpha1
kind: ApprovalTask
metadata:
  name: group-approval
  namespace: default
spec:
  approvers:
  - name: dev-team
    type: Group
    input: reject
    users:           # Users from the group are only added if the approve/reject
    - name: alice
      input: approve
    - name: bob
      input: approve
    - name: charlie
      input: reject
  numberOfApprovalsRequired: 3
  description: "Any member of dev-team can approve"
```

## Advanced Examples

### 1. Mixed User and Group Approval

```yaml
apiVersion: openshift-pipelines.org/v1alpha1
kind: ApprovalTask
metadata:
  name: mixed-approval
  namespace: default
spec:
  approvers:
  - name: tech-lead
    type: User
    input: pending
  - name: qa-team
    type: Group
    input: pending
    users:
    - name: tester1
      input: approve
    - name: tester2
      input: approve
  - name: security-team
    type: Group
    input: pending
  numberOfApprovalsRequired: 3
  description: "Requires tech lead + QA team member + security team member"
```

### 2. Using in Pipeline

```yaml
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: deployment-pipeline
spec:
  tasks:
  - name: build
    taskRef:
      name: build-task
  - name: test
    taskRef:
      name: test-task
    runAfter: [build]
  - name: approval-gate
    taskRef:
      apiVersion: openshift-pipelines.org/v1alpha1
      kind: ApprovalTask
    params:
    - name: approvers
      value:
      - alice
      - bob
      - group:security-team
    - name: numberOfApprovalsRequired
      value: "2"
    - name: description
      value: "Approve deployment to production"
    runAfter: [test]
  - name: deploy
    taskRef:
      name: deploy-task
    runAfter: [approval-gate]
```

## Status Fields

The ApprovalTask status provides detailed information about the approval process:

### Progress Tracking

```yaml
status:
  state: pending
  approvalsRequired: 3      # Total approvals needed
  approvalsReceived: 1      # Current approvals count
  approvers: 
  - alice
  - bob
  - security-team
  approversResponse:
  - name: alice
    type: User
    response: approved
    message: "LGTM for production deployment"
  - name: bob
    type: User
    response: approved
```

### Approved State

```yaml
status:
  state: approved
  approvalsRequired: 2
  approvalsReceived: 2
  approversResponse:
  - name: alice
    type: User
    response: approved
    message: "Code looks good"
  - name: security-team
    type: Group
    response: approved
    message: "Security review passed"
    groupMembers:
    - name: security-lead
      response: approved
      message: "No security issues found"
```

### Rejected State

```yaml
status:
  state: rejected
  approvalsRequired: 2
  approvalsReceived: 0
  approversResponse:
  - name: alice
    type: User
    response: rejected
    message: "Found critical bugs in the code"
```
