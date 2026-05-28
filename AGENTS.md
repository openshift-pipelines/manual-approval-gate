# Manual Approval Gate

Tekton Custom Task controller, validating webhook, and CLI (`tkn-approvaltask`)
that adds manual approval checkpoints to Tekton PipelineRuns.

---

## Build & Test Commands

```bash
# Build
go build -v ./...

# Unit tests (no cluster needed)
go test -v ./...

# Single-file lint and type-check (fast, no cluster)
go vet ./path/to/file.go
go build ./path/to/package/...

# Lint all
make lint

# Commit message lint (optional: pip install gitlint)
make gitlint                              # lint last commit
make gitlint GITLINT_COMMITS=origin/main..HEAD  # lint all commits on branch

# E2E tests (requires Docker; spins up Kind + Tekton)
./test/e2e-test.sh

# Deploy to cluster (requires ko + cluster with Tekton Pipelines)
make apply                     # Kubernetes
make TARGET=openshift apply    # OpenShift

# Code generation — after changing pkg/apis/ types
hack/update-codegen.sh

# Dependency update — after go.mod changes
hack/update-deps.sh
```

E2E tests are tagged `//go:build e2e` and live in `test/`.

---

## Key Conventions

1. **Vendored dependencies.** All builds use `-mod=vendor`. Run
   `hack/update-deps.sh` after any `go.mod` change.

2. **Knative controller runtime.** Reconcilers use `knative.dev/pkg`,
   not `controller-runtime`. Status conditions use Knative APIs.

3. **CustomRun reconciler pattern.** The controller watches Tekton `CustomRun`
   resources that reference `ApprovalTask` (apiVersion
   `openshift-pipelines.org/v1alpha1`). On each reconcile it creates or
   updates a corresponding `ApprovalTask` CR and monitors approval state.

4. **Validating webhook.** Admission endpoint `/approval-validation` enforces
   that only listed approvers can modify their own input. Uses the Kubernetes
   user identity from the admission request.

5. **ko-based images.** Container images are built with `ko`; base image is
   distroless. See `.ko.yaml` for overrides.

6. **CLI is a tkn plugin.** The `tkn-approvaltask` binary provides `list`,
   `describe`, `approve`, `reject` commands using dynamic client.

---

## Architecture

```
cmd/controller/                → CustomRun reconciler (main binary)
cmd/webhook/                   → Validating admission webhook
cmd/tkn-approvaltask/          → CLI plugin entry point
pkg/apis/approvaltask/v1alpha1/→ CRD types, validation, deepcopy
pkg/reconciler/approvaltask/   → Core reconciliation logic
pkg/reconciler/webhook/        → Admission validation logic
pkg/cli/                       → CLI commands, formatters, golden-file tests
pkg/client/                    → Generated clientset, informers, listers
config/kubernetes/             → Kubernetes manifests (CRD, RBAC, deployments)
config/openshift/              → OpenShift-specific manifests
test/                          → E2E tests and test data
```

---

## PR Conventions

- Commit messages should follow [Tekton community standards](https://github.com/tektoncd/community/blob/master/standards.md#commit-messages); run `make gitlint` locally before push when gitlint is installed.
- `go test -v ./...` must pass with zero failures.
- `go vet ./...` must be clean.
- Commits require `Signed-off-by` (DCO).

---

## Windows Checkout

`CLAUDE.md` points to `AGENTS.md`, and `.claude/skills` points to `.agents/skills`.
This works on Linux, macOS, and GitHub; on Windows, enable symlinks when cloning:

```bash
git clone -c core.symlinks=true https://github.com/openshift-pipelines/manual-approval-gate.git
```

Alternatively, set `core.symlinks=true` in your git config before checkout.

---

## Skills

- **Commit messages**: Conventional commits with component scopes,
  line length validation, DCO Signed-off-by, and Assisted-by trailers.
- **Running tests**: Unit tests, e2e tests, and golden-file CLI tests.
