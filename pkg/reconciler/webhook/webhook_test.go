package webhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
)

func makeApprovalTask(approvers []v1alpha1.ApproverDetails, nRequired int) *v1alpha1.ApprovalTask {
	return makeApprovalTaskWithDesc(approvers, nRequired, "")
}

func makeApprovalTaskWithDesc(approvers []v1alpha1.ApproverDetails, nRequired int, description string) *v1alpha1.ApprovalTask {
	return &v1alpha1.ApprovalTask{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "openshift-pipelines.org/v1alpha1",
			Kind:       "ApprovalTask",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-at",
			Namespace: "default",
		},
		Spec: v1alpha1.ApprovalTaskSpec{
			Approvers:                 approvers,
			NumberOfApprovalsRequired: nRequired,
			Description:               description,
		},
	}
}

func mustMarshal(t *testing.T, obj interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal object: %v", err)
	}
	return b
}

func makeUpdateRequest(t *testing.T, oldObj, newObj *v1alpha1.ApprovalTask, username string, groups []string) *admissionv1.AdmissionRequest {
	t.Helper()
	return &admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Kind: metav1.GroupVersionKind{
			Group:   Group,
			Version: Version,
			Kind:    Kind,
		},
		Object: runtime.RawExtension{
			Raw: mustMarshal(t, newObj),
		},
		OldObject: runtime.RawExtension{
			Raw: mustMarshal(t, oldObj),
		},
		UserInfo: authenticationv1.UserInfo{
			Username: username,
			Groups:   groups,
		},
	}
}

func newTestReconciler() *reconciler {
	return &reconciler{}
}

func TestAdmit_ApproverListImmutability(t *testing.T) {
	baseApprovers := []v1alpha1.ApproverDetails{
		{Name: "alice", Type: "User", Input: "pending"},
		{Name: "bob", Type: "User", Input: "pending"},
		{Name: "carol", Type: "User", Input: "pending"},
	}

	tests := []struct {
		name        string
		oldObj      *v1alpha1.ApprovalTask
		newObj      *v1alpha1.ApprovalTask
		username    string
		wantAllowed bool
		wantMessage string
	}{
		{
			name:   "appended approvers rejected",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask(append(
				[]v1alpha1.ApproverDetails{
					{Name: "alice", Type: "User", Input: "approve"},
					{Name: "bob", Type: "User", Input: "pending"},
					{Name: "carol", Type: "User", Input: "pending"},
				},
				v1alpha1.ApproverDetails{Name: "phantom", Type: "User", Input: "approve"},
			), 3),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.approvers list membership is immutable",
		},
		{
			name:   "removed approvers rejected",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
			}, 3),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.approvers list membership is immutable",
		},
		{
			name:   "renamed approver rejected",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "phantom", Type: "User", Input: "approve"},
			}, 3),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.approvers[2] identity (name/type) is immutable",
		},
		{
			name:   "changed approver type rejected",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "Group", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
			}, 3),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.approvers[1] identity (name/type) is immutable",
		},
		{
			name: "empty type treated as User - no false positive",
			oldObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "", Input: "pending"},
				{Name: "bob", Type: "User", Input: "pending"},
			}, 2),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
			}, 2),
			username:    "alice",
			wantAllowed: true,
		},
		{
			name:   "legitimate approval allowed",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
			}, 3),
			username:    "alice",
			wantAllowed: true,
		},
		{
			name:   "phantom approver injection blocked",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
				{Name: "x-phantom-1", Type: "User", Input: "approve"},
				{Name: "x-phantom-2", Type: "User", Input: "approve"},
			}, 3),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.approvers list membership is immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			ctx := logging.WithLogger(context.Background(), logtesting.TestLogger(t))
			r.withContext = func(context.Context) context.Context { return ctx }

			req := makeUpdateRequest(t, tt.oldObj, tt.newObj, tt.username, nil)
			resp := r.Admit(ctx, req)

			if resp.Allowed != tt.wantAllowed {
				msg := ""
				if resp.Result != nil {
					msg = resp.Result.Message
				}
				t.Errorf("Allowed = %v, want %v (message: %s)", resp.Allowed, tt.wantAllowed, msg)
			}
			if !tt.wantAllowed && tt.wantMessage != "" {
				got := ""
				if resp.Result != nil {
					got = resp.Result.Message
				}
				if got != tt.wantMessage {
					t.Errorf("message = %q, want %q", got, tt.wantMessage)
				}
			}
		})
	}
}

func TestAdmit_SpecFieldImmutability(t *testing.T) {
	baseApprovers := []v1alpha1.ApproverDetails{
		{Name: "alice", Type: "User", Input: "pending"},
		{Name: "bob", Type: "User", Input: "pending"},
		{Name: "carol", Type: "User", Input: "pending"},
	}

	tests := []struct {
		name        string
		oldObj      *v1alpha1.ApprovalTask
		newObj      *v1alpha1.ApprovalTask
		username    string
		wantAllowed bool
		wantMessage string
	}{
		{
			name:   "numberOfApprovalsRequired lowered rejected",
			oldObj: makeApprovalTask(baseApprovers, 3),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
			}, 1),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.numberOfApprovalsRequired is immutable",
		},
		{
			name:   "numberOfApprovalsRequired raised rejected",
			oldObj: makeApprovalTask(baseApprovers, 2),
			newObj: makeApprovalTask([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
			}, 3),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.numberOfApprovalsRequired is immutable",
		},
		{
			name:   "description changed rejected",
			oldObj: makeApprovalTaskWithDesc(baseApprovers, 3, "deploy to prod"),
			newObj: makeApprovalTaskWithDesc([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
			}, 3, "deploy to staging"),
			username:    "alice",
			wantAllowed: false,
			wantMessage: "spec.description is immutable",
		},
		{
			name:   "legitimate approval with unchanged spec allowed",
			oldObj: makeApprovalTaskWithDesc(baseApprovers, 3, "deploy to prod"),
			newObj: makeApprovalTaskWithDesc([]v1alpha1.ApproverDetails{
				{Name: "alice", Type: "User", Input: "approve"},
				{Name: "bob", Type: "User", Input: "pending"},
				{Name: "carol", Type: "User", Input: "pending"},
			}, 3, "deploy to prod"),
			username:    "alice",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			ctx := logging.WithLogger(context.Background(), logtesting.TestLogger(t))
			r.withContext = func(context.Context) context.Context { return ctx }

			req := makeUpdateRequest(t, tt.oldObj, tt.newObj, tt.username, nil)
			resp := r.Admit(ctx, req)

			if resp.Allowed != tt.wantAllowed {
				msg := ""
				if resp.Result != nil {
					msg = resp.Result.Message
				}
				t.Errorf("Allowed = %v, want %v (message: %s)", resp.Allowed, tt.wantAllowed, msg)
			}
			if !tt.wantAllowed && tt.wantMessage != "" {
				got := ""
				if resp.Result != nil {
					got = resp.Result.Message
				}
				if got != tt.wantMessage {
					t.Errorf("message = %q, want %q", got, tt.wantMessage)
				}
			}
		})
	}
}

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
