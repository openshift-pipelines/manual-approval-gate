package list

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

func TestListApprovalTasks(t *testing.T) {
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "at-2",
				Namespace: "foo",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "tekton",
						Input: "approve",
						Type:  "User",
					},
					{
						Name:  "cli",
						Input: "approve",
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
						Response: "approved",
					},
					{
						Name:     "cli",
						Type:     "User",
						Response: "approved",
					},
				},
				State: "approved",
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
				State: "pending",
			},
		},
	}

	approvaltasksMultipleNs := []*v1alpha1.ApprovalTask{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mango",
				Namespace: "test-1",
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apple",
				Namespace: "test-2",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "tekton",
						Input: "approve",
						Type:  "User",
					},
					{
						Name:  "cli",
						Input: "approve",
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
						Response: "approved",
					},
					{
						Name:     "cli",
						Type:     "User",
						Response: "approved",
					},
				},
				State: "approved",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "banana",
				Namespace: "test-3",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{
						Name:  "tekton",
						Input: "pending",
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

	dc2, err := testDynamic.Client(
		cb.UnstructuredV1alpha1(approvaltasksMultipleNs[0], "v1alpha1"),
		cb.UnstructuredV1alpha1(approvaltasksMultipleNs[1], "v1alpha1"),
		cb.UnstructuredV1alpha1(approvaltasksMultipleNs[2], "v1alpha1"),
	)
	if err != nil {
		t.Errorf("unable to create dynamic client: %v", err)
	}

	tests := []struct {
		name      string
		command   *cobra.Command
		args      []string
		wantError bool
	}{
		{
			name:      "no approval tasks found",
			command:   command(t, approvaltasks, ns, dc),
			args:      []string{"list", "-n", "invalid"},
			wantError: true,
		},
		{
			name:      "all in namespace",
			command:   command(t, approvaltasks, ns, dc),
			args:      []string{"list", "-n", "foo"},
			wantError: false,
		},
		{
			name:      "in all namespaces",
			command:   command(t, approvaltasksMultipleNs, ns, dc2),
			args:      []string{"list", "--all-namespaces"},
			wantError: false,
		},
	}

	for _, td := range tests {
		t.Run(td.name, func(t *testing.T) {
			output, err := test.ExecuteCommand(td.command, td.args...)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err != nil && !td.wantError {
				t.Errorf("Unexpected error: %v", err)
			}

			golden.Assert(t, output, strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
		})
	}

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

func TestRejectedWithGroups(t *testing.T) {
	tests := []struct {
		name     string
		at       *v1alpha1.ApprovalTask
		expected int
	}{
		{
			name: "group with multiple rejections",
			at: &v1alpha1.ApprovalTask{
				Status: v1alpha1.ApprovalTaskStatus{
					ApproversResponse: []v1alpha1.ApproverState{
						{
							Name: "qa-team",
							Type: "Group",
							GroupMembers: []v1alpha1.GroupMemberState{
								{Name: "tester1", Response: "rejected"},
								{Name: "tester2", Response: "rejected"},
								{Name: "tester3", Response: "approved"},
							},
						},
					},
				},
			},
			expected: 2, // 2 rejections from group members
		},
		{
			name: "mixed user and group rejections",
			at: &v1alpha1.ApprovalTask{
				Status: v1alpha1.ApprovalTaskStatus{
					ApproversResponse: []v1alpha1.ApproverState{
						{
							Name:     "direct-user",
							Type:     "User",
							Response: "rejected",
						},
						{
							Name: "admin-group",
							Type: "Group",
							GroupMembers: []v1alpha1.GroupMemberState{
								{Name: "admin1", Response: "rejected"},
								{Name: "admin2", Response: "approved"},
							},
						},
					},
				},
			},
			expected: 2, // 1 direct user + 1 group member rejected = 2
		},
		{
			name: "no rejections",
			at: &v1alpha1.ApprovalTask{
				Status: v1alpha1.ApprovalTaskStatus{
					ApproversResponse: []v1alpha1.ApproverState{
						{
							Name:     "user1",
							Type:     "User",
							Response: "approved",
						},
					},
				},
			},
			expected: 0, // no rejections
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rejected(tt.at)
			if result != tt.expected {
				t.Errorf("rejected() = %d, expected %d", result, tt.expected)
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
