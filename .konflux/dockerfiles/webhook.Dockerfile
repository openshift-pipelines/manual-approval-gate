ARG GO_BUILDER=brew.registry.redhat.io/rh-osbs/openshift-golang-builder:v1.23
ARG RUNTIME=registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:92b1d5747a93608b6adb64dfd54515c3c5a360802db4706765ff3d8470df6290

FROM $GO_BUILDER AS builder

WORKDIR /go/src/github.com/openshift-pipelines/manual-approval-gate
COPY . .
RUN set -e; for f in patches/*.patch; do echo ${f}; [[ -f ${f} ]] || continue; git apply ${f}; done
ENV GODEBUG="http2server=0"
RUN git rev-parse HEAD > /tmp/HEAD
ENV GOEXPERIMENT=strictfipsruntime
RUN go build -ldflags="-X 'knative.dev/pkg/changeset.rev=$(cat /tmp/HEAD)'" -mod=vendor -tags disable_gcp -tags strictfipsruntime -v -o /tmp/manual-approval-gate-webhook \
    ./cmd/webhook

FROM $RUNTIME
ARG VERSION=manual-approval-gate-webhook-1-18

ENV KO_APP=/ko-app

COPY --from=builder /tmp/manual-approval-gate-webhook ${KO_APP}/manual-approval-gate-webhook

LABEL \
    com.redhat.component="openshift-pipelines-manual-approval-gate-rhel9-container" \
    name="openshift-pipelines/pipelines-manual-approval-gate-webhook-rhel9" \
    cpe="cpe:/a:redhat:openshift_pipelines:1.18::el9" \
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
