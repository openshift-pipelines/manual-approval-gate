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
)

// TestCheckOtherUsersForInvalidChangesPreservesMembers verifies that omission is
// treated as a change while complete lists and member reordering remain valid.
func TestCheckOtherUsersForInvalidChangesPreservesMembers(t *testing.T) {
	// Cases exercise the comparison independently of the current user's input check.
	cases := []struct {
		name    string                 // name identifies the attempted update.
		users   []v1alpha1.UserDetails // users is the replacement group member list.
		groups  []string               // groups contains the authenticated memberships.
		allowed bool                   // allowed is the expected preservation decision.
	}{
		{name: "replace-other-member", users: []v1alpha1.UserDetails{{Name: "bob", Input: "approve"}}, groups: []string{"reviewers"}},
		{name: "omit-all-members", groups: []string{"reviewers"}},
		{name: "empty-members", users: []v1alpha1.UserDetails{}, groups: []string{"reviewers"}},
		{name: "change-other-input", users: []v1alpha1.UserDetails{{Name: "alice", Input: "reject"}, {Name: "bob", Input: "approve"}}, groups: []string{"reviewers"}},
		{name: "append-own-member", users: []v1alpha1.UserDetails{{Name: "alice", Input: "approve"}, {Name: "bob", Input: "approve"}}, groups: []string{"reviewers"}, allowed: true},
		{name: "prepend-own-member", users: []v1alpha1.UserDetails{{Name: "bob", Input: "approve"}, {Name: "alice", Input: "approve"}}, groups: []string{"reviewers"}, allowed: true},
		{name: "preserve-all-members", users: []v1alpha1.UserDetails{{Name: "alice", Input: "approve"}}, groups: []string{"reviewers"}, allowed: true},
		{name: "add-on-behalf-of-another", users: []v1alpha1.UserDetails{{Name: "alice", Input: "approve"}, {Name: "carol", Input: "approve"}}, groups: []string{"reviewers"}},
		{name: "add-without-membership", users: []v1alpha1.UserDetails{{Name: "alice", Input: "approve"}, {Name: "bob", Input: "approve"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Both group-level inputs remain unchanged to isolate member preservation.
			oldApprovers := []v1alpha1.ApproverDetails{{Name: "reviewers", Type: "Group", Input: "approve", Users: []v1alpha1.UserDetails{{Name: "alice", Input: "approve"}}}}
			newApprovers := []v1alpha1.ApproverDetails{{Name: "reviewers", Type: "Group", Input: "approve", Users: tc.users}}
			request := &admissionv1.AdmissionRequest{UserInfo: authenticationv1.UserInfo{Username: "bob", Groups: tc.groups}}
			if got := CheckOtherUsersForInvalidChanges(oldApprovers, newApprovers, request); got != tc.allowed {
				t.Errorf("preservation decision = %v, want %v", got, tc.allowed)
			}
		})
	}
}

// TestAdmitGroupApprovalPreservation exercises decoding, authorization and the
// complete admission path with the object that Kubernetes produces after a patch.
func TestAdmitGroupApprovalPreservation(t *testing.T) {
	// Reuse the existing denial message so clients need no new error handling.
	const preservationDenied = "User can only update their own approval input"
	// Cases include a valid self-vote in negative preservation tests: a rejection
	// must reach the other-member check instead of failing for lack of self-change.
	cases := []struct {
		name       string   // name describes the client update shape.
		usersJSON  string   // usersJSON is the raw members field; empty means omitted.
		individual bool     // individual adds a valid direct self-vote for omission cases.
		ownPending bool     // ownPending adds an undecided member for own-input updates.
		groups     []string // groups overrides membership when non-nil.
		state      string   // state supplies an existing terminal state when nonempty.
		duplicate  bool     // duplicate adds a reconciled prior decision by the caller.
		allowed    bool     // allowed is the expected admission result.
		message    string   // message identifies the rejection reason, not an outage.
	}{
		{name: "stale-full-list-replaces-alice", usersJSON: `[{"name":"bob","input":"approve"}]`, message: preservationDenied},
		{name: "members-omitted-with-valid-self-vote", individual: true, message: preservationDenied},
		{name: "members-null-with-valid-self-vote", usersJSON: `null`, individual: true, message: preservationDenied},
		{name: "members-empty-with-valid-self-vote", usersJSON: `[]`, individual: true, message: preservationDenied},
		{name: "change-alice-with-valid-self-vote", usersJSON: `[{"name":"alice","input":"reject"},{"name":"bob","input":"approve"}]`, message: preservationDenied},
		{name: "fresh-full-list-preserves-alice", usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"}]`, allowed: true},
		{name: "frontend-prepends-caller", usersJSON: `[{"name":"bob","input":"approve"},{"name":"alice","input":"approve"}]`, allowed: true},
		{name: "own-pending-input-can-change", ownPending: true, usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"}]`, allowed: true},
		{name: "user-and-group-self-vote", individual: true, usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"}]`, allowed: true},
		{name: "add-other-member-with-self-vote", usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"},{"name":"carol","input":"approve"}]`, message: preservationDenied},
		{name: "unauthorized-member", groups: []string{}, usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"}]`, message: "User does not exist in the approval list"},
		{name: "final-state-remains-immutable", state: "approved", usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"}]`, message: "ApprovalTask has already reached it's final state"},
		{name: "duplicate-vote-remains-denied", duplicate: true, usersJSON: `[{"name":"alice","input":"approve"},{"name":"bob","input":"approve"}]`, message: "User has already approved"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Keep the task below quorum before the update, including duplicate cases.
			oldTask := &v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					NumberOfApprovalsRequired: 3,
					Approvers:                 []v1alpha1.ApproverDetails{{Name: "reviewers", Type: "Group", Input: "approve", Users: []v1alpha1.UserDetails{{Name: "alice", Input: "approve"}}}},
				},
				Status: v1alpha1.ApprovalTaskStatus{State: tc.state},
			}
			if tc.individual {
				oldTask.Spec.Approvers = append([]v1alpha1.ApproverDetails{{Name: "bob", Type: "User", Input: "pending"}}, oldTask.Spec.Approvers...)
			}
			// groupIndex follows the optional direct approver without changing identity rules.
			groupIndex := len(oldTask.Spec.Approvers) - 1
			if tc.ownPending {
				oldTask.Spec.Approvers[groupIndex].Users = append(oldTask.Spec.Approvers[groupIndex].Users, v1alpha1.UserDetails{Name: "bob", Input: "pending"})
			}
			if tc.duplicate {
				oldTask.Status.ApproversResponse = []v1alpha1.ApproverState{{Name: "reviewers", Type: "Group", GroupMembers: []v1alpha1.GroupMemberState{{Name: "bob", Response: "approved"}}}}
			}
			// Build the incoming object as JSON to distinguish null, [] and omission.
			newTask := oldTask.DeepCopy()
			newTask.Spec.Approvers[groupIndex].Users = nil
			if tc.individual {
				newTask.Spec.Approvers[0].Input = "approve"
			}
			newBytes, err := json.Marshal(newTask)
			if err != nil {
				t.Fatal(err)
			}
			var incoming map[string]interface{} // incoming carries the literal users representation.
			if err := json.Unmarshal(newBytes, &incoming); err != nil {
				t.Fatal(err)
			}
			if tc.usersJSON != "" {
				var users interface{} // users preserves explicit null and empty arrays.
				if err := json.Unmarshal([]byte(tc.usersJSON), &users); err != nil {
					t.Fatal(err)
				}
				incoming["spec"].(map[string]interface{})["approvers"].([]interface{})[groupIndex].(map[string]interface{})["users"] = users
			}
			newBytes, err = json.Marshal(incoming)
			if err != nil {
				t.Fatal(err)
			}
			oldBytes, err := json.Marshal(oldTask)
			if err != nil {
				t.Fatal(err)
			}
			// Default to an authenticated member, with an explicit unauthorized case.
			groups := tc.groups
			if groups == nil {
				groups = []string{"reviewers"}
			}
			request := &admissionv1.AdmissionRequest{
				Kind:      metav1.GroupVersionKind{Group: Group, Version: Version, Kind: Kind},
				Operation: admissionv1.Update,
				UserInfo:  authenticationv1.UserInfo{Username: "bob", Groups: groups},
				Object:    runtime.RawExtension{Raw: newBytes},
				OldObject: runtime.RawExtension{Raw: oldBytes},
			}
			response := (&reconciler{}).Admit(context.Background(), request)
			if response.Allowed != tc.allowed {
				t.Errorf("Allowed = %v, want %v (result: %+v)", response.Allowed, tc.allowed, response.Result)
			}
			if !tc.allowed && (response.Result == nil || response.Result.Message != tc.message) {
				t.Errorf("rejection = %+v, want message %q", response.Result, tc.message)
			}
		})
	}
}
