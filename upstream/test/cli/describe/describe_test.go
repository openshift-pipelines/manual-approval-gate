//go:build e2e
// +build e2e

package describe

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/test/cli"
	"github.com/openshift-pipelines/manual-approval-gate/test/client"
	"github.com/openshift-pipelines/manual-approval-gate/test/resources"
	"github.com/stretchr/testify/assert"
	"gotest.tools/v3/golden"
)

func TestApprovalTaskDescribeCommand(t *testing.T) {
	tknApprovaltask, err := cli.NewTknApprovalTaskRunner()
	assert.Nil(t, err)

	clients := client.Setup(t, "default")

	cr := resources.Create(t, clients, "./testdata/cr-1.yaml")

	_, err = resources.WaitForApprovalTaskCreation(clients.ApprovalTaskClient, cr.GetName(), cr.GetNamespace())
	if err != nil {
		t.Fatal("Failed to get the approval task")
	}

	res := tknApprovaltask.MustSucceed(t, "describe", cr.GetName(), "-n", "test-5")
	golden.Assert(t, res.Stdout(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
}
