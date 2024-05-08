package approve

import (
	"fmt"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/test"
	cb "github.com/openshift-pipelines/manual-approval-gate/pkg/test/builder"
	testDynamic "github.com/openshift-pipelines/manual-approval-gate/pkg/test/dynamic"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func TestApproveApprovalTask(t *testing.T) {
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
						Input: "pending",
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
				State: "pending",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "at-2",
				Namespace: "foo",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "tekton",
						Input: "pending",
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
				State: "pending",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "at-3",
				Namespace: "foo",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "tekton",
						Input: "pending",
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
				State: "pending",
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
		cb.UnstructuredV1alpha1(approvaltasks[1], "v1alpha1"),
		cb.UnstructuredV1alpha1(approvaltasks[2], "v1alpha1"),
	)
	if err != nil {
		t.Errorf("unable to create dynamic client: %v", err)
	}

	tests := []struct {
		name           string
		command        *cobra.Command
		args           []string
		expectedOutput string
		wantError      bool
	}{
		{
			name:           "approve approval task",
			command:        command(t, approvaltasks, ns, dc, "tekton"),
			args:           []string{"at-1", "-n", "foo"},
			expectedOutput: "ApprovalTask at-1 is approved in foo namespace\n",
			wantError:      false,
		},
		{
			name:           "invalid username",
			command:        command(t, approvaltasks, ns, dc, "test-user"),
			args:           []string{"at-2", "-n", "foo"},
			expectedOutput: "Error: failed to approve approvalTask from namespace foo: Approver: test-user, is not present in the approvers list\n",
			wantError:      true,
		},
		{
			name:           "approvaltask not found",
			command:        command(t, approvaltasks, ns, dc, "tekton"),
			args:           []string{"at-3", "-n", "test"},
			expectedOutput: fmt.Sprintf("Error: failed to approve approvalTask from namespace %s: approvaltasks.openshift-pipelines.org \"%s\" not found\n", "test", "at-3"),
			wantError:      true,
		},
	}

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			output, err := test.ExecuteCommand(td.command, td.args...)
			if err != nil && !td.wantError {
				t.Errorf("Unexpected error: %v", err)
			}

			if output != td.expectedOutput {
				t.Errorf("Expected output to be %q, but got %q", td.expectedOutput, output)
			}
		})
	}
}

func command(t *testing.T, approvaltasks []*v1alpha1.ApprovalTask, ns []*corev1.Namespace, dc dynamic.Interface, username string) *cobra.Command {
	cs, _ := test.SeedTestData(t, test.Data{Approvaltasks: approvaltasks, Namespaces: ns})
	p := &test.Params{ApprovalTask: cs.ApprovalTask, Kube: cs.Kube, Dynamic: dc, Username: username}
	cs.ApprovalTask.Resources = cb.APIResourceList("v1alpha1", []string{"approvaltask"})

	return Command(p)
}
