#!/usr/bin/env bash

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
  ko apply -f config/kubernetes
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

# Define the function to create a Kind cluster.
create_kind_cluster() {
   # Create a Kind cluster configuration file.
  kind create cluster --image kindest/node:v1.25.0

  # Check if the Kind cluster was created successfully.
  if [ $? -eq 0 ]; then
    echo "Kind cluster created successfully."
  else
    echo "Failed to create Kind cluster."
    exit 1
  fi
}

main() {
  # create_kind_cluster

  # Script entry point.
  export RELEASE_YAML="https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.51.0/release.yaml"
  # install_pipeline_crd

  # install_manual_approval_crd

  wait_until_pods_running tekton-pipelines || fail_test "Manual Approval did not come up"

  failed=0

  KUBECONFIG=${KUBECONFIG:-"${HOME}/.kube/config"}
  KUBECONFIG_PARAM=${KUBECONFIG:+"--kubeconfig $KUBECONFIG"}
  # Run the integration tests
  echo "Running Go e2e tests"
  go test -v -count=1 -tags=e2e -timeout=20m ./test/e2e_test.go ${KUBECONFIG_PARAM}  || failed=1
}

main