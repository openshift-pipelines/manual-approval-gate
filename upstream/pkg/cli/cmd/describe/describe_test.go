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
						Type:  "User",
					},
					{
						Name:  "cli",
						Input: "pending",
						Type:  "User",
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
						Type:     "User",
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

func TestDescribeApprovalTaskWithGroups(t *testing.T) {
	approvaltasks := []*v1alpha1.ApprovalTask{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "at-group",
				Namespace: "foo",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "admin-group",
						Input: "approve",
						Type:  "Group",
					},
					{
						Name:  "dev-team",
						Input: "reject",
						Type:  "Group",
					},
					{
						Name:  "alice",
						Input: "pending",
						Type:  "User",
					},
				},
				NumberOfApprovalsRequired: 3,
			},
			Status: v1alpha1.ApprovalTaskStatus{
				Approvers: []string{
					"admin-group",
					"dev-team",
					"alice",
				},
				ApproversResponse: []v1alpha1.ApproverState{
					{
						Name:     "admin-group",
						Type:     "Group",
						Response: "approved",
						GroupMembers: []v1alpha1.GroupMemberState{
							{
								Name:     "bob",
								Response: "approved",
								Message:  "LGTM",
							},
							{
								Name:     "charlie",
								Response: "approved",
							},
						},
					},
					{
						Name:     "dev-team",
						Type:     "Group",
						Response: "rejected",
						GroupMembers: []v1alpha1.GroupMemberState{
							{
								Name:     "david",
								Response: "rejected",
								Message:  "Needs more testing",
							},
						},
					},
				},
				State: "approved",
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
	args := []string{"at-group", "-n", "foo"}

	output, err := test.ExecuteCommand(c, args...)
	golden.Assert(t, output, strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
}

// Test individual functions for group functionality
func TestPendingApprovalsWithGroups(t *testing.T) {
	tests := []struct {
		name     string
		at       *v1alpha1.ApprovalTask
		expected int
	}{
		{
			name: "group with multiple members responded",
			at: &v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					NumberOfApprovalsRequired: 3,
				},
				Status: v1alpha1.ApprovalTaskStatus{
					ApproversResponse: []v1alpha1.ApproverState{
						{
							Name: "admin-group",
							Type: "Group",
							GroupMembers: []v1alpha1.GroupMemberState{
								{Name: "alice", Response: "approved"},
								{Name: "bob", Response: "rejected"},
							},
						},
					},
				},
			},
			expected: 1, // 3 required - 2 responded = 1 pending
		},
		{
			name: "mixed user and group responses",
			at: &v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					NumberOfApprovalsRequired: 4,
				},
				Status: v1alpha1.ApprovalTaskStatus{
					ApproversResponse: []v1alpha1.ApproverState{
						{
							Name:     "direct-user",
							Type:     "User",
							Response: "approved",
						},
						{
							Name: "dev-team",
							Type: "Group",
							GroupMembers: []v1alpha1.GroupMemberState{
								{Name: "charlie", Response: "approved"},
								{Name: "david", Response: "approved"},
							},
						},
					},
				},
			},
			expected: 1, // 4 required - 3 responded = 1 pending
		},
		{
			name: "no responses",
			at: &v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					NumberOfApprovalsRequired: 2,
				},
				Status: v1alpha1.ApprovalTaskStatus{
					ApproversResponse: []v1alpha1.ApproverState{},
				},
			},
			expected: 2, // 2 required - 0 responded = 2 pending
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pendingApprovals(tt.at)
			if result != tt.expected {
				t.Errorf("pendingApprovals() = %d, expected %d", result, tt.expected)
			}
		})
	}
}

func command(t *testing.T, approvaltasks []*v1alpha1.ApprovalTask, ns []*corev1.Namespace, dc dynamic.Interface) *cobra.Command {
	cs, _ := test.SeedTestData(t, test.Data{Approvaltasks: approvaltasks, Namespaces: ns})
	p := &test.Params{ApprovalTask: cs.ApprovalTask, Kube: cs.Kube, Dynamic: dc}
	cs.ApprovalTask.Resources = cb.APIResourceList("v1alpha1", []string{"approvaltask"})

	return Command(p)
}
