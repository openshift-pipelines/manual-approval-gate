//go:build e2e
// +build e2e

package list

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

func TestApprovalTaskListCommand(t *testing.T) {
	tknApprovaltask, err := cli.NewTknApprovalTaskRunner()
	assert.Nil(t, err)

	t.Run("No approvaltask found", func(t *testing.T) {
		expected := "No ApprovalTasks found\n"

		res := tknApprovaltask.MustSucceed(t, "list", "-n", "foo")
		assert.Equal(t, expected, res.Stdout())
	})

	t.Run("list approvaltask in some other namespace", func(t *testing.T) {
		clients := client.Setup(t, "default")

		// Create custom run with pending state
		_ = resources.Create(t, clients, "./testdata/cr-1.yaml")

		// Create custom run which is reaches the rejected state
		cr2 := resources.Create(t, clients, "./testdata/cr-2.yaml")
		approvers := []resources.Approver{
			{
				Name:  "foo",
				Input: "approve",
			},
			{
				Name:  "tekton",
				Input: "reject",
			},
		}
		resources.Update(t, clients, cr2, approvers, "rejected")

		// Create custom run which is reaches the approved state
		cr3 := resources.Create(t, clients, "./testdata/cr-3.yaml")
		approvers3 := []resources.Approver{
			{
				Name:  "foo",
				Input: "approve",
			},
			{
				Name:  "bar",
				Input: "approve",
			},
		}
		resources.Update(t, clients, cr3, approvers3, "approved")

		res := tknApprovaltask.MustSucceed(t, "list", "-n", "test-1")
		golden.Assert(t, res.Stdout(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
	})
}
