#!/usr/bin/env bash

export KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
export KUBECONFIG=${HOME}/.kube/config.${KIND_CLUSTER_NAME}
kind=$(type -p kind)
TMPD=$(mktemp -d /tmp/.GITXXXX)
REG_PORT='5000'
REG_NAME='kind-registry'
INSTALL_FROM_RELEASE=
SUDO=sudo

source $(dirname $0)/../vendor/github.com/tektoncd/plumbing/scripts/e2e-tests.sh

function start_registry() {
    running="$(docker inspect -f '{{.State.Running}}' ${REG_NAME} 2>/dev/null || echo false)"

    if [[ ${running} != "true" ]];then
        docker rm -f kind-registry || true
        docker run \
               -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" \
               -e REGISTRY_HTTP_SECRET=secret \
               --name "${REG_NAME}" \
               registry:2
    fi
}

function reinstall_kind() {
	${SUDO} $kind delete cluster --name ${KIND_CLUSTER_NAME} || true
	sed "s,%DOCKERCFG%,${HOME}/.docker/config.json," test/kind.yaml > ${TMPD}/kconfig.yaml

       cat <<EOF >> ${TMPD}/kconfig.yaml
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REG_PORT}"]
    endpoint = ["http://${REG_NAME}:5000"]
EOF

	${SUDO} ${kind} create cluster --name ${KIND_CLUSTER_NAME} --config  ${TMPD}/kconfig.yaml
	mkdir -p $(dirname ${KUBECONFIG})
	${SUDO} ${kind} --name ${KIND_CLUSTER_NAME} get kubeconfig > ${KUBECONFIG}


    docker network connect "kind" "${REG_NAME}" 2>/dev/null || true
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
}

function install_pipeline_crd() {
  local latestreleaseyaml
  echo ">> Deploying Tekton Pipelines"
  if [[ -n ${RELEASE_YAML} ]];then
  latestreleaseyaml=${RELEASE_YAML}
  else
  latestreleaseyaml=$(curl -s https://api.github.com/repos/tektoncd/pipeline/releases|python -c "import sys, json;x=json.load(sys.stdin);ass=x[0]['assets'];print([ x['browser_download_url'] for x in ass if x['name'] == 'release.yaml'][0])")
  fi
  [[ -z ${latestreleaseyaml} ]] && fail_test "Could not get latest released release.yaml"
  kubectl apply -f ${latestreleaseyaml} ||
  fail_test "Tekton pipeline installation failed"

  # Make sure that eveything is cleaned up in the current namespace.
  for res in pipelineresources tasks pipelines taskruns pipelineruns; do
  kubectl delete --ignore-not-found=true ${res}.tekton.dev --all
  done

  # Wait for pods to be running in the namespaces we are deploying to
  wait_until_pods_running tekton-pipelines || fail_test "Tekton Pipeline did not come up"
}

function install_manual_approval_crd() {
  echo ">> Deploying Manual Approval Gate"
  env KO_DOCKER_REPO=localhost:5000 ko apply -f config/kubernetes --sbom=none -B >/dev/null
  wait_until_pods_running tekton-pipelines || fail_test "Manual Approval did not come up"
}

function wait_until_pods_running() {
  echo -n "Waiting until all pods in namespace $1 are up"
  for i in {1..150}; do  # timeout after 5 minutes
    local pods="$(kubectl get pods --no-headers -n $1 2>/dev/null)"
    # All pods must be running
    local not_running=$(echo "${pods}" | grep -v Running | grep -v Completed | wc -l)
    if [[ -n "${pods}" && ${not_running} -eq 0 ]]; then
      local all_ready=1
      while read pod ; do
        local status=(`echo -n ${pod} | cut -f2 -d' ' | tr '/' ' '`)
        # All containers must be ready
        [[ -z ${status[0]} ]] && all_ready=0 && break
        [[ -z ${status[1]} ]] && all_ready=0 && break
        [[ ${status[0]} -lt 1 ]] && all_ready=0 && break
        [[ ${status[1]} -lt 1 ]] && all_ready=0 && break
        [[ ${status[0]} -ne ${status[1]} ]] && all_ready=0 && break
      done <<< $(echo "${pods}" | grep -v Completed)
      if (( all_ready )); then
        echo -e "\nAll pods are up:\n${pods}"
        return 0
      fi
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for pods to come up\n${pods}"
  return 1
}

main() {
  start_registry
	reinstall_kind
  install_pipeline_crd
  install_manual_approval_crd

  echo "Running Go e2e tests"
  go test -v -count=1 -tags=e2e -timeout=20m ./test/e2e_test.go ${KUBECONFIG_PARAM} || fail_test "E2E test failed....."
  kubectl delete customruns.tekton.dev -n default --all

  echo "Running CLI e2e tests"
  kubectl create ns test-1
  kubectl create ns test-2
  kubectl create ns test-3
  kubectl create ns test-4
  kubectl create ns test-5

  go build -o tkn-approvaltask github.com/openshift-pipelines/manual-approval-gate/cmd/tkn-approvaltask
  export TEST_CLIENT_BINARY="${PWD}/tkn-approvaltask"

  go test -v -count=1 -tags=e2e -timeout=20m ./test/cli/... ${KUBECONFIG_PARAM} || fail_test "E2E test failed....."

  kubectl delete ns test-1
  kubectl delete ns test-2
  kubectl delete ns test-3
  kubectl create ns test-4
  kubectl create ns test-5

  success
}

main
