#!/usr/bin/env bash

RELEASE_VERSION="$1"

DOCKER_CMD=${DOCKER_CMD:-docker}

BINARIES="ko gh"

info() {
  echo "INFO: $@"
}

err() {
  echo "ERROR: $@"
}

getReleaseVersion() {
  [[ -z ${RELEASE_VERSION} ]] && {
      read -r -e -p "Enter a target release (i.e: v0.1.2): " RELEASE_VERSION
      [[ -z ${RELEASE_VERSION} ]] && {
        echo "no target release"
        exit 1
      }
    }
    [[ ${RELEASE_VERSION} =~ v[0-9]+\.[0-9]*\.[0-9]+ ]] || {
      echo "invalid version provided, need to match v\d+\.\d+\.\d+"
      exit 1
    }
}

buildImageAndGenerateReleaseYaml() {
  	info Creating Manual Approval Gate Release Yaml for Kubernetes
  	echo "------------------------------------------"
    ko resolve -f config/kubernetes -t ${RELEASE_VERSION} > release-kubernetes.yaml || {
      err 'release build failed'
      return 1
    }

    sed -i "s/version: \"devel\"/version: \"$RELEASE_VERSION\"/g" release-kubernetes.yaml

    echo "============================================="
    info Creating Manual Approval Gate Release Yaml for Openshift
    echo "------------------------------------------"
    ko resolve -f config/openshift -t ${RELEASE_VERSION} > release-openshift.yaml || {
          err 'release build failed'
          return 1
    }

    sed -i "s/version: \"devel\"/version: \"$RELEASE_VERSION\"/g" release-openshift.yaml

    echo "------------------------------------------"
}

createNewPreRelease() {
  echo; echo 'Creating New Manual Approval Gate Pre-Release :'

  gh repo set-default git@github.com:openshift-pipelines/manual-approval-gate.git

  gh release create ${RELEASE_VERSION} --title "Pre-release Version '${RELEASE_VERSION}'" --notes "Description of the prerelease" --draft --prerelease

  gh release upload ${RELEASE_VERSION} release-kubernetes.yaml release-openshift.yaml
}

createNewBranchAndPush() {
  git checkout -b release-${RELEASE_VERSION}

  git push origin release-${RELEASE_VERSION}
}

main() {

  # Check if all required command exists
  for b in ${BINARIES};do
      type -p ${b} >/dev/null || { echo "'${b}' need to be avail"; exit 1 ;}
  done

  # Ask the release version to build images
  getReleaseVersion

  # Build images for db-migration, api and ui
  echo "********************************************"
  info        Build the Images for Manual Approval Gate
  echo "********************************************"
  buildImageAndGenerateReleaseYaml

  # Create a new pre-release
  echo "********************************************"
  info            Create New PreRelease
  echo "********************************************"
  createNewPreRelease

  echo "********************************************"
  info            Create New Branch And Push
  echo "********************************************"
  createNewBranchAndPush

  echo "************************************************************"
  echo    Release Created for Manual Approval Gate successfully
  echo "************************************************************"
}

main $@
