//go:build e2e
// +build e2e

package approve

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/test/cli"
	"github.com/openshift-pipelines/manual-approval-gate/test/client"
	"github.com/openshift-pipelines/manual-approval-gate/test/resources"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApprovalTaskRejectCommand(t *testing.T) {
	tknApprovaltask, err := cli.NewTknApprovalTaskRunner()
	assert.Nil(t, err)

	clients := client.Setup(t, "default")

	t.Run("approve-approvaltask", func(t *testing.T) {
		cr := resources.Create(t, clients, "./testdata/cr-1.yaml")

		_, err := resources.WaitForApprovalTaskCreation(clients.ApprovalTaskClient, cr.GetName(), cr.GetNamespace())
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}

		res := tknApprovaltask.MustSucceed(t, "reject", cr.GetName(), "-n", "test-4")

		_, err = resources.WaitForApproverResponseUpdate(clients.ApprovalTaskClient, cr, "kubernetes-admin", "rejected")
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}

		approvalTask, err := clients.ApprovalTaskClient.ApprovalTasks(cr.GetNamespace()).Get(context.TODO(), cr.GetName(), metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}

		assert.Equal(t, 1, len(approvalTask.Status.ApproversResponse))
		assert.Equal(t, "ApprovalTask at-1 is rejected in test-4 namespace\n", res.Stdout())
		assert.Equal(t, "kubernetes-admin", approvalTask.Status.ApproversResponse[0].Name)
		assert.Equal(t, "rejected", approvalTask.Status.ApproversResponse[0].Response)
	})
}
