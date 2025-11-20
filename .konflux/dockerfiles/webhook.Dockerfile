ARG GO_BUILDER=brew.registry.redhat.io/rh-osbs/openshift-golang-builder:v1.23
ARG RUNTIME=registry.redhat.io/ubi8/ubi:latest@sha256:7d7ca86d832d1dc7aba4583414475c15686291b1c2cf75fe63ca03526c3b89ae

FROM $GO_BUILDER AS builder

WORKDIR /go/src/github.com/openshift-pipelines/manual-approval-gate
COPY . .
RUN set -e; for f in patches/*.patch; do echo ${f}; [[ -f ${f} ]] || continue; git apply ${f}; done
ENV GODEBUG="http2server=0"
ENV GOEXPERIMENT=strictfipsruntime
RUN git rev-parse HEAD > /tmp/HEAD
RUN CGO_ENABLED=0 \
    go build -ldflags="-X 'knative.dev/pkg/changeset.rev=$(cat /tmp/HEAD)'" -mod=vendor -tags disable_gcp,strictfipsruntime -v -o /tmp/manual-approval-gate-webhook \
    ./cmd/webhook

FROM $RUNTIME
ARG VERSION=manual-approval-gate-webhook-1.17.2

ENV KO_APP=/ko-app

COPY --from=builder /tmp/manual-approval-gate-webhook ${KO_APP}/manual-approval-gate-webhook

LABEL \
    com.redhat.component="openshift-pipelines-manual-approval-gate-rhel8-container" \
    name="openshift-pipelines/pipelines-manual-approval-gate-webhook-rhel8" \
    cpe="cpe:/a:redhat:openshift_pipelines:1.17::el8" \
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
