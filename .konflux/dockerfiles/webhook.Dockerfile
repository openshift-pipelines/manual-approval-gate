ARG GO_BUILDER=brew.registry.redhat.io/rh-osbs/openshift-golang-builder:v1.22
ARG RUNTIME=registry.redhat.io/ubi8/ubi:latest@sha256:8bd1b6306f8164de7fb0974031a0f903bd3ab3e6bcab835854d3d9a1a74ea5db

FROM $GO_BUILDER AS builder

WORKDIR /go/src/github.com/openshift-pipelines/manual-approval-gate
COPY . .
RUN set -e; for f in patches/*.patch; do echo ${f}; [[ -f ${f} ]] || continue; git apply ${f}; done
ENV GODEBUG="http2server=0"
RUN git rev-parse HEAD > /tmp/HEAD
RUN go build -ldflags="-X 'knative.dev/pkg/changeset.rev=$(cat /tmp/HEAD)'" -mod=vendor -tags disable_gcp -v -o /tmp/manual-approval-gate-webhook \
    ./cmd/webhook

FROM $RUNTIME
ARG VERSION=manual-approval-gate-webhook-1.15.3

ENV KO_APP=/ko-app

COPY --from=builder /tmp/manual-approval-gate-webhook ${KO_APP}/manual-approval-gate-webhook

LABEL \
    com.redhat.component="openshift-pipelines-manual-approval-gate-rhel8-container" \
    name="openshift-pipelines/pipelines-manual-approval-gate-rhel8" \
    version=$VERSION \
    summary="Red Hat OpenShift Pipelines Manual Approval Gate" \
    maintainer="pipelines-extcomm@redhat.com" \
    description="Red Hat OpenShift Pipelines Manual Approval Gate" \
    io.k8s.display-name="Red Hat OpenShift Pipelines Manual Approval Gate" \
    io.k8s.description="Red Hat OpenShift Pipelines Manual Approval Gate" \
    io.openshift.tags="pipelines,tekton,openshift"

RUN groupadd -r -g 65532 nonroot && useradd --no-log-init -r -u 65532 -g nonroot nonroot
USER 65532

ENTRYPOINT ["/ko-app/manual-approval-gate-webhook"]