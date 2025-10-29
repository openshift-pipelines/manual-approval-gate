package webhook

import (
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
)

// TestIsUserApprovalChanged_UserCanModifyOwnUserAndGroupApproval tests that a user
// who is both listed as an individual User approver and is a member of a Group approver
// can modify either their User entry or their Group entry.
func TestIsUserApprovalChanged_UserCanModifyOwnUserAndGroupApproval(t *testing.T) {
	tests := []struct {
		name            string
		oldApprovers    []v1alpha1.ApproverDetails
		newApprovers    []v1alpha1.ApproverDetails
		username        string
		userGroups      []string
		expectedChanged bool
		expectedError   bool
		description     string
	}{
		{
			name: "user modifies their own User entry only",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone modifies their own User entry from pending to approve",
		},
		{
			name: "user modifies their Group entry only",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone (who is in team-with-someone) modifies the Group entry from pending to approve",
		},
		{
			name: "user modifies both their User entry and Group entry",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone modifies both their User entry and Group entry",
		},
		{
			name: "user has not modified anything",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "admin",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: false,
			expectedError:   false,
			description:     "User someone has not changed any approvals",
		},
		{
			name: "user provides invalid input value",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "invalid",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: false,
			expectedError:   true,
			description:     "User someone provides invalid input value",
		},
		{
			name: "user is not in the group and cannot modify group entry",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "g-other",
					Type:  "Group",
					Input: "pending",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "g-other",
					Type:  "Group",
					Input: "approve",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: false,
			expectedError:   false,
			description:     "User someone is not in g-other group and should not be able to modify it",
		},
		{
			name: "user modifies group entry with multiple groups",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
				},
				{
					Name:  "system:authenticated",
					Type:  "Group",
					Input: "pending",
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
				},
				{
					Name:  "system:authenticated",
					Type:  "Group",
					Input: "pending",
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone", "system:authenticated"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone modifies one of their group entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &admissionv1.AdmissionRequest{
				UserInfo: authenticationv1.UserInfo{
					Username: tt.username,
					Groups:   tt.userGroups,
				},
			}

			changed, err := IsUserApprovalChanged(tt.oldApprovers, tt.newApprovers, request)

			if tt.expectedError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectedError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}
			if changed != tt.expectedChanged {
				t.Errorf("%s: expected changed=%v, got changed=%v", tt.description, tt.expectedChanged, changed)
			}
		})
	}
}

// TestIsUserApprovalChanged_GroupUsersList tests scenarios where users are explicitly
// listed in the group's users list
func TestIsUserApprovalChanged_GroupUsersList(t *testing.T) {
	tests := []struct {
		name            string
		oldApprovers    []v1alpha1.ApproverDetails
		newApprovers    []v1alpha1.ApproverDetails
		username        string
		userGroups      []string
		expectedChanged bool
		expectedError   bool
		description     string
	}{
		{
			name: "user modifies their input in group users list",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
					Users: []v1alpha1.UserDetails{
						{
							Name:  "someone",
							Input: "pending",
						},
					},
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
					Users: []v1alpha1.UserDetails{
						{
							Name:  "someone",
							Input: "approve",
						},
					},
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone modifies their input in group users list",
		},
		{
			name: "user adds themselves to group users list",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
					Users: []v1alpha1.UserDetails{},
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
					Users: []v1alpha1.UserDetails{
						{
							Name:  "someone",
							Input: "approve",
						},
					},
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone adds themselves to group users list",
		},
		{
			name: "user modifies both group-level input and their individual input in users list",
			oldApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "pending",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "pending",
					Users: []v1alpha1.UserDetails{
						{
							Name:  "someone",
							Input: "pending",
						},
					},
				},
			},
			newApprovers: []v1alpha1.ApproverDetails{
				{
					Name:  "someone",
					Type:  "User",
					Input: "approve",
				},
				{
					Name:  "team-with-someone",
					Type:  "Group",
					Input: "approve",
					Users: []v1alpha1.UserDetails{
						{
							Name:  "someone",
							Input: "approve",
						},
					},
				},
			},
			username:        "someone",
			userGroups:      []string{"team-with-someone"},
			expectedChanged: true,
			expectedError:   false,
			description:     "User someone modifies their User entry, group-level input, and their individual input in users list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &admissionv1.AdmissionRequest{
				UserInfo: authenticationv1.UserInfo{
					Username: tt.username,
					Groups:   tt.userGroups,
				},
			}

			changed, err := IsUserApprovalChanged(tt.oldApprovers, tt.newApprovers, request)

			if tt.expectedError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectedError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}
			if changed != tt.expectedChanged {
				t.Errorf("%s: expected changed=%v, got changed=%v", tt.description, tt.expectedChanged, changed)
			}
		})
	}
}
