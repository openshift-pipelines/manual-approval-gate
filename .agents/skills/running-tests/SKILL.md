---
name: running-tests
description: >-
  Run unit tests, e2e tests, and golden-file CLI tests for
  manual-approval-gate. Use when asked to "run tests", "test this change",
  "verify tests pass", or when investigating test failures.
license: Apache-2.0
compatibility: Requires go; e2e requires docker, kind, ko, kubectl
metadata:
  project: manual-approval-gate
  allowed-tools: Bash(go test:*) Bash(make:*) Read Grep Glob
---

# Running Tests

manual-approval-gate has three test layers: unit tests, CLI golden-file
tests, and end-to-end tests requiring a live cluster.

## Unit Tests (fast, no cluster)

```bash
# All unit tests
go test -v ./...

# With race detector
go test -v -race ./...

# Single package
go test -v ./pkg/reconciler/approvaltask/...

# Specific test function
go test -v -run TestReconcile ./pkg/reconciler/approvaltask/...
```

Key unit test files:

| File | What it tests |
|------|---------------|
| `pkg/reconciler/approvaltask/utils_test.go` | Reconciler utility logic |
| `pkg/cli/cmd/list/list_test.go` | List command (golden files) |
| `pkg/cli/cmd/describe/describe_test.go` | Describe command |
| `pkg/cli/cmd/approve/approve_test.go` | Approve command |
| `pkg/cli/cmd/reject/reject_test.go` | Reject command |

## CLI Golden-File Tests

CLI commands use golden-file testing. Expected outputs live in
`pkg/cli/cmd/*/testdata/*.golden`.

To update golden files after intentional output changes:

```bash
go test -v -run TestList ./pkg/cli/cmd/list/... -update
```

The `-update` flag rewrites `.golden` files with actual output.

## E2E Tests (requires Docker + Kind)

The full e2e suite spins up a Kind cluster with Tekton Pipelines installed:

```bash
./test/e2e-test.sh
```

This script:
1. Starts a local Docker registry
2. Creates a Kind cluster (`test/kind.yaml`)
3. Installs Tekton Pipelines from the latest release
4. Deploys manual-approval-gate via `ko apply -f config/kubernetes`
5. Runs controller e2e: `go test -v -count=1 -tags=e2e -timeout=20m ./test/e2e_test.go`
6. Runs CLI e2e: `go test -v -count=1 -tags=e2e -timeout=20m ./test/cli/...`

### Running e2e against an existing cluster

If you already have a cluster with Tekton and the approval gate deployed:

```bash
go test -v -count=1 -tags=e2e -timeout=20m ./test/e2e_test.go
go test -v -count=1 -tags=e2e -timeout=20m ./test/cli/...
```

### E2E test scenarios

The e2e suite covers (`test/e2e_test.go`):
- Approval flow (single approver)
- Rejection flow
- Group-based approvers
- Timeout behavior
- Multiple approvers with quorum

## Makefile Shortcuts

```bash
make test-unit   # go test -v -race ./...
make lint        # go vet ./...
make fmt         # gofmt -l -w .
```

## Troubleshooting

- **Tests hang**: Check if e2e tests are accidentally running without the
  `e2e` build tag (they need a cluster)
- **Golden file mismatch**: Run with `-update` flag, then `git diff` to
  review changes before committing
- **Webhook tests fail**: Ensure admission webhook certs are valid in
  test fixtures
