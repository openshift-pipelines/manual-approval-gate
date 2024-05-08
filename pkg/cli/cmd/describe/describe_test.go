package describe

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/test"
	cb "github.com/openshift-pipelines/manual-approval-gate/pkg/test/builder"
	testDynamic "github.com/openshift-pipelines/manual-approval-gate/pkg/test/dynamic"
	"github.com/spf13/cobra"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func TestDescribeApprovalTask(t *testing.T) {
	approvaltasks := []*v1alpha1.ApprovalTask{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "at-1",
				Namespace: "foo",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "tekton",
						Input: "reject",
					},
					{
						Name:  "cli",
						Input: "pending",
					},
				},
				NumberOfApprovalsRequired: 2,
			},
			Status: v1alpha1.ApprovalTaskStatus{
				Approvers: []string{
					"tekton",
					"cli",
				},
				ApproversResponse: []v1alpha1.ApproverState{
					{
						Name:     "tekton",
						Response: "rejected",
					},
				},
				State: "rejected",
			},
		},
	}

	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace",
			},
		},
	}

	dc, err := testDynamic.Client(
		cb.UnstructuredV1alpha1(approvaltasks[0], "v1alpha1"),
	)
	if err != nil {
		t.Errorf("unable to create dynamic client: %v", err)
	}

	c := command(t, approvaltasks, ns, dc)
	args := []string{"at-1", "-n", "foo"}

	output, err := test.ExecuteCommand(c, args...)
	golden.Assert(t, output, strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
}

func TestDescribeApprovalTaskNotFound(t *testing.T) {
	ns := []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "namespace",
			},
		},
	}

	dc, err := testDynamic.Client()
	if err != nil {
		t.Errorf("unable to create dynamic client: %v", err)
	}

	c := command(t, []*v1alpha1.ApprovalTask{}, ns, dc)
	args := []string{"at-1", "-n", "foo"}

	output, err := test.ExecuteCommand(c, args...)

	expectedOutput := "Error: failed to Get ApprovalTasks at-1 from foo namespace\n"
	if output != expectedOutput {
		t.Errorf("Expected output to be %q, but got %q", expectedOutput, output)
	}
}

func command(t *testing.T, approvaltasks []*v1alpha1.ApprovalTask, ns []*corev1.Namespace, dc dynamic.Interface) *cobra.Command {
	cs, _ := test.SeedTestData(t, test.Data{Approvaltasks: approvaltasks, Namespaces: ns})
	p := &test.Params{ApprovalTask: cs.ApprovalTask, Kube: cs.Kube, Dynamic: dc}
	cs.ApprovalTask.Resources = cb.APIResourceList("v1alpha1", []string{"approvaltask"})

	return Command(p)
}
