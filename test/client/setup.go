package client

import (
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/test/utils"
	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	pkgTest "knative.dev/pkg/test"
)

// Setup creates the client objects needed in the e2e tests.
func Setup(t *testing.T, namespace string) *utils.Clients {
	clients, err := utils.NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		namespace)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	return clients
}
