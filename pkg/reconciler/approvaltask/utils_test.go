/*
Copyright 2023 The OpenShift Pipelines Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package approvaltask

import (
	"context"
	"strings"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCheckCustomRunReferencesApprovalTaskValidReferences(t *testing.T) {
	// Create a sample CustomRun with correct APIVersion and Kind references
	run := &v1beta1.CustomRun{
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: approvaltaskv1alpha1.SchemeGroupVersion.String(),
				Kind:       approvaltask.ControllerName,
			},
		},
	}

	// Call the function and expect no error
	err := checkCustomRunReferencesApprovalTask(run)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	assert.NoError(t, err)
}

// Negative unit test for checkCustomRunReferencesApprovalTask function
func TestCheckCustomRunReferencesApprovalTaskInvalidReferences(t *testing.T) {
	// Create a sample CustomRun with incorrect APIVersion and Kind references
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
		},
	}

	// Call the function and expect an error
	err := checkCustomRunReferencesApprovalTask(run)
	if err == nil {
		t.Errorf("Expected an error, but got nil")
	}

	// Check if the error message matches the expected error message
	expectedErrorMsg := "Received control for a Run foo/bar that does not reference a ApprovalTask custom CRD"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message: %s, but got: %s", expectedErrorMsg, err.Error())
	}
}

func TestPropagateApprovalTaskLabelsAndAnnotations(t *testing.T) {
	// Create a sample CustomRun with incorrect APIVersion and Kind references
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
		},
	}

	approvalTaskMeta := &metav1.ObjectMeta{
		Name: "foo-bar",
	}

	propagateApprovalTaskLabelsAndAnnotations(run, approvalTaskMeta)

	expectedValue := run.Labels["openshift-pipelines.org/approvaltask"]
	assert.Equal(t, expectedValue, "foo-bar")
	assert.Equal(t, len(run.Labels), 1)
}

func TestCreateApprovalTask(t *testing.T) {
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
			Params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("foo", "bar", "tekton"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("2"),
				},
			},
		},
	}

	client := fake.NewSimpleClientset()

	approvalTask, err := createApprovalTask(context.TODO(), client, run)
	if err != nil {
		t.Fatalf("createApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)

	assert.Equal(t, "bar", approvalTask.Name, "ApprovalTask name should match")
	assert.Equal(t, "foo", approvalTask.Namespace, "ApprovalTask namespace should match")

	assert.Equal(t, 3, len(approvalTask.Spec.Approvers), "Expected 3 approvals")
	assert.Equal(t, 2, approvalTask.Spec.NumberOfApprovalsRequired, "Expected approvalsRequired to be 2")

	expectedNames := []string{"foo", "bar", "tekton"}
	for _, approver := range approvalTask.Spec.Approvers {
		assert.Contains(t, expectedNames, approver.Name, "Approval name should be in the expected list")
		assert.Equal(t, "pending", approver.Input, "Approval InputValue should be 'pending'")
		assert.Equal(t, "User", approver.Type, "Approval Type should be 'User'")
	}

	assert.Equal(t, "pending", approvalTask.Status.State, "ApprovalState should be 'pending'")
}

func TestCreateApprovalTaskWithoutApprovalsRequiredProvided(t *testing.T) {
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
			Params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("foo", "bar", "tekton"),
				},
			},
		},
	}

	client := fake.NewSimpleClientset()

	approvalTask, err := createApprovalTask(context.TODO(), client, run)
	if err != nil {
		t.Fatalf("createApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)

	assert.Equal(t, 1, approvalTask.Spec.NumberOfApprovalsRequired, "Expected approvalsRequired to be 2")

	assert.Equal(t, approvalTask.Status.State, "pending", "ApprovalState should be in `wait`")
}

func TestUpdateApprovalTaskFalseState(t *testing.T) {
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
			Params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("foo", "bar", "tekton"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("2"),
				},
			},
		},
	}

	client := fake.NewSimpleClientset()

	approvalTask, err := createApprovalTask(context.TODO(), client, run)
	if err != nil {
		t.Fatalf("createApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)
	assert.Equal(t, approvalTask.Status.State, "pending", "ApprovalState should be in `wait`")

	approvalTask.Spec.Approvers[0].Input = "reject"

	at, err := client.OpenshiftpipelinesV1alpha1().ApprovalTasks(approvalTask.Namespace).Update(context.TODO(), &approvalTask, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed updating the input value for for")
	}

	at1, err := updateApprovalState(context.TODO(), client, at)
	if err != nil {
		t.Fatalf("updateApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)

	assert.Equal(t, at1.Status.State, "rejected", "ApprovalState should be in `false`")
	assert.Equal(t, len(at1.Status.ApproversResponse), 1, "foo has approved it")
}

func TestUpdateApprovalTaskWaitState(t *testing.T) {
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
			Params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("foo", "bar", "tekton"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("2"),
				},
			},
		},
	}

	client := fake.NewSimpleClientset()

	approvalTask, err := createApprovalTask(context.TODO(), client, run)
	if err != nil {
		t.Fatalf("createApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)
	assert.Equal(t, approvalTask.Status.State, "pending", "ApprovalState should be in `wait`")

	approvalTask.Spec.Approvers[0].Input = "approve"
	at, err := client.OpenshiftpipelinesV1alpha1().ApprovalTasks(approvalTask.Namespace).Update(context.TODO(), &approvalTask, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed updating the input value for for")
	}

	at1, err := updateApprovalState(context.TODO(), client, at)
	if err != nil {
		t.Fatalf("updateApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)

	assert.Equal(t, at1.Status.State, "pending", "ApprovalState should be in `wait`")
	assert.Equal(t, len(at1.Status.ApproversResponse), 1, "foo has approved it")
	assert.Equal(t, at1.Status.ApproversResponse[0].Name, "foo", "foo has approved it")
	assert.Equal(t, at1.Status.ApproversResponse[0].Response, "approved", "foo has approved it")
}

func TestUpdateApprovalTaskTrueState(t *testing.T) {
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
			Params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("foo", "bar", "tekton"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("2"),
				},
			},
		},
	}

	client := fake.NewSimpleClientset()

	approvalTask, err := createApprovalTask(context.TODO(), client, run)
	if err != nil {
		t.Fatalf("createApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)
	assert.Equal(t, approvalTask.Status.State, "pending", "ApprovalState should be in `wait`")

	approvalTask.Spec.Approvers[0].Input = "approve"
	approvalTask.Spec.Approvers[1].Input = "approve"

	at, err := client.OpenshiftpipelinesV1alpha1().ApprovalTasks(approvalTask.Namespace).Update(context.TODO(), &approvalTask, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed updating the input value for for")
	}

	at1, err := updateApprovalState(context.TODO(), client, at)
	if err != nil {
		t.Fatalf("updateApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)

	assert.Equal(t, at1.Status.State, "approved", "ApprovalState should be in `true`")
	assert.Equal(t, len(at1.Status.ApproversResponse), 2)
}

func TestUpdateApprovalTaskWithNoApprovalsProvided(t *testing.T) {
	run := &v1beta1.CustomRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
		},
		Spec: v1beta1.CustomRunSpec{
			CustomRef: &v1beta1.TaskRef{
				APIVersion: "wrong-api-version",
				Kind:       "wrong-kind",
			},
		},
	}

	client := fake.NewSimpleClientset()

	approvalTask, err := createApprovalTask(context.TODO(), client, run)
	if err != nil {
		t.Fatalf("createApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)
	assert.Equal(t, approvalTask.Status.State, "pending", "ApprovalState should be in `wait`")
	assert.Equal(t, approvalTask.Spec.NumberOfApprovalsRequired, 1, "ApprovalsRequired should be 1`")

	approvals := v1alpha1.ApproverDetails{
		Name:  "foo",
		Input: "approve",
		Type:  "User",
	}
	approvalTask.Spec.Approvers = append(approvalTask.Spec.Approvers, approvals)

	at, err := client.OpenshiftpipelinesV1alpha1().ApprovalTasks(approvalTask.Namespace).Update(context.TODO(), &approvalTask, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed updating the input value for for")
	}

	at1, err := updateApprovalState(context.TODO(), client, at)
	if err != nil {
		t.Fatalf("updateApprovalTask returned an error: %v", err)
	}
	assert.Equal(t, nil, err)

	assert.Equal(t, at1.Status.State, "approved", "ApprovalState should be in `true`")
	assert.Equal(t, len(at1.Status.ApproversResponse), 1, "foo has approved it")
}

func TestApprovalTaskHasFalseInputWithOneApproval(t *testing.T) {
	approvaltask := v1alpha1.ApprovalTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.ApprovalTaskSpec{
			Approvers: []v1alpha1.ApproverDetails{
				{
					Name:  "apple",
					Input: "reject",
				},
				{
					Name:  "banana",
					Input: "pending",
				},
				{
					Name:  "mango",
					Input: "approve",
				},
			},
			NumberOfApprovalsRequired: 2,
		},
		Status: v1alpha1.ApprovalTaskStatus{},
	}

	got := approvalTaskHasFalseInput(approvaltask)
	assert.Equal(t, true, got)
}

func TestApprovalTaskHasTrueInputWithAllApprovals(t *testing.T) {
	approvaltask := v1alpha1.ApprovalTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.ApprovalTaskSpec{
			Approvers: []v1alpha1.ApproverDetails{
				{
					Name:  "foo",
					Input: "pending",
					Type:  "User",
				},
				{
					Name:  "bar",
					Input: "approve",
					Type:  "User",
				},
				{
					Name:  "tekton",
					Input: "approve",
					Type:  "User",
				},
			},
			NumberOfApprovalsRequired: 2,
		},
		Status: v1alpha1.ApprovalTaskStatus{},
	}

	got := approvalTaskHasTrueInput(approvaltask)
	assert.Equal(t, true, got)
}

func TestApprovalTaskHasTrueInputWithSomeApprovals(t *testing.T) {
	approvaltask := v1alpha1.ApprovalTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Spec: v1alpha1.ApprovalTaskSpec{
			Approvers: []v1alpha1.ApproverDetails{
				{
					Name:  "foo",
					Input: "pending",
				},
				{
					Name:  "bar",
					Input: "pending",
				},
				{
					Name:  "tekton",
					Input: "approve",
				},
			},
			NumberOfApprovalsRequired: 2,
		},
		Status: v1alpha1.ApprovalTaskStatus{},
	}

	got := approvalTaskHasTrueInput(approvaltask)
	assert.Equal(t, false, got)
}

func TestApprovalTaskHasTrueInputWithGroup(t *testing.T) {
	// Test case with Group that has individual user approvals
	approvalTask := v1alpha1.ApprovalTask{
		Spec: v1alpha1.ApprovalTaskSpec{
			NumberOfApprovalsRequired: 2,
			Approvers: []v1alpha1.ApproverDetails{
				{
					Name:  "user1",
					Input: "pending",
					Type:  "User",
				},
				{
					Name:  "dev-team",
					Input: "approve",
					Type:  "Group",
					Users: []v1alpha1.UserDetails{
						{
							Name:  "alice",
							Input: "approve",
						},
						{
							Name:  "bob",
							Input: "approve",
						},
					},
				},
			},
		},
	}

	result := approvalTaskHasTrueInput(approvalTask)
	assert.True(t, result, "Should return true when group has 2 approvals and requirement is 2")
}

func TestApprovalTaskHasFalseInput(t *testing.T) {
	// Test case with rejection
	approvalTask := v1alpha1.ApprovalTask{
		Spec: v1alpha1.ApprovalTaskSpec{
			Approvers: []v1alpha1.ApproverDetails{
				{
					Name:  "user1",
					Input: "approve",
					Type:  "User",
				},
				{
					Name:  "user2",
					Input: "reject",
					Type:  "User",
				},
			},
		},
	}

	result := approvalTaskHasFalseInput(approvalTask)
	assert.True(t, result, "Should return true when any approver has rejected")
}

// Test the validation functions for parameter validation
func TestValidateApproverParameter(t *testing.T) {
	tests := []struct {
		name        string
		paramValue  string
		paramIndex  int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid user",
			paramValue:  "user1",
			paramIndex:  0,
			expectError: false,
		},
		{
			name:        "valid group",
			paramValue:  "group:approver-group",
			paramIndex:  0,
			expectError: false,
		},
		{
			name:        "invalid group with space after colon",
			paramValue:  "group: approver-group3",
			paramIndex:  2,
			expectError: true,
			errorMsg:    "approvers[2]: invalid group format 'group: approver-group3' - use 'group:groupname' format (remove spaces around colon)",
		},
		{
			name:        "invalid group with space before colon",
			paramValue:  "group :approver-group3",
			paramIndex:  1,
			expectError: true,
			errorMsg:    "approvers[1]: invalid group format 'group :approver-group3' - use 'group:groupname' format (remove spaces around colon)",
		},
		{
			name:        "empty approver",
			paramValue:  "",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: approver name cannot be empty",
		},
		{
			name:        "empty approver with spaces",
			paramValue:  "   ",
			paramIndex:  1,
			expectError: true,
			errorMsg:    "approvers[1]: approver name cannot be empty",
		},
		{
			name:        "user with spaces",
			paramValue:  "user with spaces",
			paramIndex:  0,
			expectError: false,
			// Spaces are now allowed for LDAP/AD integration
		},
		{
			name:        "user with colon - ServiceAccount format",
			paramValue:  "system:serviceaccount:default:builder",
			paramIndex:  0,
			expectError: false,
			// ServiceAccounts and other K8s identities with colons are allowed
		},
		{
			name:        "user with colon - OAuth format", 
			paramValue:  "oauth:alice",
			paramIndex:  0,
			expectError: false,
			// OAuth users with colons are allowed
		},
		{
			name:        "valid group format",
			paramValue:  "group:dev-team",
			paramIndex:  0, 
			expectError: false,
			// This is valid group syntax, should pass
		},
		{
			name:        "empty group name",
			paramValue:  "group:",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: invalid group format 'group:' - group name cannot be empty after 'group:'",
		},
		{
			name:        "group name with spaces",
			paramValue:  "group:approver group",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: group name 'approver group' cannot contain spaces",
		},
		{
			name:        "user with special characters",
			paramValue:  "user@#$%",
			paramIndex:  0,
			expectError: false,
			// Special characters are now allowed for enterprise LDAP/AD integration
		},
		{
			name:        "valid user with various characters",
			paramValue:  "user1.test_user@example-org",
			paramIndex:  0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApproverParameter(tt.paramValue, tt.paramIndex)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCustomRunParameters(t *testing.T) {
	tests := []struct {
		name        string
		params      []v1beta1.Param
		expectError bool
		errorMsg    string
	}{
		{
			name: "invalid group with space",
			params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("user1", "group: approver-group3"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("1"),
				},
			},
			expectError: true,
			errorMsg:    "invalid approvers parameter: approvers[1]: invalid group format 'group: approver-group3' - use 'group:groupname' format (remove spaces around colon)",
		},
		{
			name: "invalid numberOfApprovalsRequired not a number",
			params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("user1"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("not-a-number"),
				},
			},
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: 'not-a-number' is not a valid integer",
		},
		{
			name: "invalid numberOfApprovalsRequired zero",
			params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("user1"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("0"),
				},
			},
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: must be greater than 0, got 0",
		},
		{
			name: "no approvers",
			params: []v1beta1.Param{
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("1"),
				},
			},
			expectError: true,
			errorMsg:    "no valid approvers found - at least one approver is required",
		},
		{
			name: "numberOfApprovalsRequired exceeds approvers - should pass",
			params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("user1", "group:large-team"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("5"),
				},
			},
			expectError: false,
			// Note: This should pass because group:large-team might have many members
			// Group membership is resolved at runtime, not validation time
		},
		{
			name: "malformed group as object (YAML parsing issue)",
			params: []v1beta1.Param{
				{
					Name: "approvers",
					Value: v1beta1.ParamValue{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"user1", "user2", "group:valid-group"},
						ObjectVal: map[string]string{
							"group": "example", // This simulates {"group": "example"} from YAML
						},
					},
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("1"),
				},
			},
			expectError: true,
			errorMsg:    "invalid approvers parameter: approvers[3]: invalid group format {\"group\":\"example\"} - use 'group:example' format instead",
		},
		{
			name: "other object format",
			params: []v1beta1.Param{
				{
					Name: "approvers",
					Value: v1beta1.ParamValue{
						Type:     v1beta1.ParamTypeArray,
						ArrayVal: []string{"user1"},
						ObjectVal: map[string]string{
							"invalid": "format",
						},
					},
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("1"),
				},
			},
			expectError: true,
			errorMsg:    "invalid approvers parameter: approvers[1]: invalid object format {\"invalid\":\"format\"} - approver must be a string, not an object",
		},
		{
			name: "valid parameters",
			params: []v1beta1.Param{
				{
					Name:  "approvers",
					Value: *v1beta1.NewArrayOrString("user1", "group:approver-group"),
				},
				{
					Name:  "numberOfApprovalsRequired",
					Value: *v1beta1.NewArrayOrString("1"),
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &v1beta1.CustomRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-run",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.CustomRunSpec{
					Params: tt.params,
				},
			}

			err := ValidateCustomRunParameters(run)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateGroupSyntax tests the validateGroupSyntax function directly
func TestValidateGroupSyntax(t *testing.T) {
	tests := []struct {
		name        string
		paramValue  string
		paramIndex  int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid group",
			paramValue:  "group:dev-team",
			paramIndex:  0,
			expectError: false,
		},
		{
			name:        "empty group name",
			paramValue:  "group:",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: invalid group format 'group:' - group name cannot be empty after 'group:'",
		},
		{
			name:        "group name with spaces",
			paramValue:  "group:dev team",
			paramIndex:  1,
			expectError: true,
			errorMsg:    "approvers[1]: group name 'dev team' cannot contain spaces",
		},
		{
			name:        "group name with colon",
			paramValue:  "group:dev:team",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: group name 'dev:team' cannot contain colons",
		},
		{
			name:        "group name with only whitespace",
			paramValue:  "group:   ",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: invalid group format 'group:   ' - group name cannot be empty after 'group:'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGroupSyntax(tt.paramValue, tt.paramIndex)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateUserSyntax tests the validateUserSyntax function directly
func TestValidateUserSyntax(t *testing.T) {
	tests := []struct {
		name        string
		paramValue  string
		paramIndex  int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid user",
			paramValue:  "user1",
			paramIndex:  0,
			expectError: false,
		},
		{
			name:        "empty user",
			paramValue:  "",
			paramIndex:  0,
			expectError: true,
			errorMsg:    "approvers[0]: username cannot be empty",
		},
		{
			name:        "user with only whitespace",
			paramValue:  "   ",
			paramIndex:  1,
			expectError: true,
			errorMsg:    "approvers[1]: username cannot be empty",
		},
		{
			name:        "user with spaces",
			paramValue:  "user with spaces",
			paramIndex:  0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserSyntax(tt.paramValue, tt.paramIndex)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateApprovalsRequired tests the validateApprovalsRequired function directly
func TestValidateApprovalsRequired(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid positive number",
			value:       "1",
			expectError: false,
		},
		{
			name:        "valid large number",
			value:       "100",
			expectError: false,
		},
		{
			name:        "zero",
			value:       "0",
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: must be greater than 0, got 0",
		},
		{
			name:        "negative number",
			value:       "-1",
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: must be greater than 0, got -1",
		},
		{
			name:        "not a number",
			value:       "abc",
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: 'abc' is not a valid integer",
		},
		{
			name:        "empty string",
			value:       "",
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: '' is not a valid integer",
		},
		{
			name:        "float number",
			value:       "1.5",
			expectError: true,
			errorMsg:    "invalid numberOfApprovalsRequired parameter: '1.5' is not a valid integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApprovalsRequired(tt.value)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestParseApproversList tests the parseApproversList function
func TestParseApproversList(t *testing.T) {
	tests := []struct {
		name            string
		param           v1beta1.Param
		expectError     bool
		expectedCount   int
		expectedApprovers []string
	}{
		{
			name: "array format",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString("user1", "user2", "group:dev-team"),
			},
			expectError:     false,
			expectedCount:   3,
			expectedApprovers: []string{"user1", "user2", "group:dev-team"},
		},
		{
			name: "string JSON array format",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString(`["user1", "user2"]`),
			},
			expectError:     false,
			expectedCount:   2,
			expectedApprovers: []string{"user1", "user2"},
		},
		{
			name: "object format",
			param: v1beta1.Param{
				Name: "approvers",
				Value: v1beta1.ParamValue{
					ObjectVal: map[string]string{
						"group": "dev-team",
					},
				},
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "invalid JSON string",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString(`invalid json`),
			},
			expectError:   true,
			expectedCount: 0,
		},
		{
			name: "string not an array",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString(`{"key": "value"}`),
			},
			expectError:   true,
			expectedCount: 0,
		},
		{
			name: "empty array",
			param: v1beta1.Param{
				Name: "approvers",
				Value: v1beta1.ParamValue{
					Type:     v1beta1.ParamTypeArray,
					ArrayVal: []string{},
				},
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var validationErrors []string
			approverList := parseApproversList(tt.param, &validationErrors)
			
			if tt.expectError {
				assert.Greater(t, len(validationErrors), 0, "Expected validation errors")
			} else {
				assert.Equal(t, tt.expectedCount, len(approverList), "Approver count should match")
				if len(tt.expectedApprovers) > 0 {
					for i, expected := range tt.expectedApprovers {
						if i < len(approverList) {
							if str, ok := approverList[i].(string); ok {
								assert.Equal(t, expected, str)
							}
						}
					}
				}
			}
		})
	}
}

// TestValidateApproversParam tests the validateApproversParam function
func TestValidateApproversParam(t *testing.T) {
	tests := []struct {
		name          string
		param         v1beta1.Param
		expectedCount int
		expectErrors  bool
	}{
		{
			name: "valid approvers",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString("user1", "user2", "group:dev-team"),
			},
			expectedCount: 3,
			expectErrors:  false,
		},
		{
			name: "invalid approver with space in group",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString("user1", "group: dev-team"),
			},
			expectedCount: 1,
			expectErrors:  true,
		},
		{
			name: "empty approver",
			param: v1beta1.Param{
				Name:  "approvers",
				Value: *v1beta1.NewArrayOrString("user1", ""),
			},
			expectedCount: 1,
			expectErrors:  true,
		},
		{
			name: "object approver",
			param: v1beta1.Param{
				Name: "approvers",
				Value: v1beta1.ParamValue{
					ObjectVal: map[string]string{
						"group": "dev-team",
					},
				},
			},
			expectedCount: 0,
			expectErrors:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, errors := validateApproversParam(tt.param)
			assert.Equal(t, tt.expectedCount, count, "Approver count should match")
			if tt.expectErrors {
				assert.Greater(t, len(errors), 0, "Expected validation errors")
			} else {
				assert.Equal(t, 0, len(errors), "Should not have validation errors")
			}
		})
	}
}

// TestValidateMalformedObjectApprover tests the validateMalformedObjectApprover function
func TestValidateMalformedObjectApprover(t *testing.T) {
	tests := []struct {
		name         string
		approver     map[string]interface{}
		index        int
		expectError  bool
		errorContains string
	}{
		{
			name: "group object",
			approver: map[string]interface{}{
				"group": "dev-team",
			},
			index:        0,
			expectError:  true,
			errorContains: "invalid group format",
		},
		{
			name: "invalid object",
			approver: map[string]interface{}{
				"invalid": "format",
			},
			index:        1,
			expectError:  true,
			errorContains: "invalid object format",
		},
		{
			name: "group with non-string value",
			approver: map[string]interface{}{
				"group": 123,
			},
			index:        0,
			expectError:  true,
			errorContains: "invalid group specification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var validationErrors []string
			validateMalformedObjectApprover(tt.approver, tt.index, &validationErrors)
			
			if tt.expectError {
				assert.Greater(t, len(validationErrors), 0, "Expected validation errors")
				if tt.errorContains != "" {
					found := false
					for _, err := range validationErrors {
						if strings.Contains(err, tt.errorContains) {
							found = true
							break
						}
					}
					assert.True(t, found, "Error should contain: %s", tt.errorContains)
				}
			}
		})
	}
}

// TestStoreApprovalTaskSpec tests the storeApprovalTaskSpec function
func TestStoreApprovalTaskSpec(t *testing.T) {
	tests := []struct {
		name              string
		status            *v1alpha1.ApprovalTaskRunStatus
		spec              *v1alpha1.ApprovalTaskSpec
		shouldStore       bool
		expectedStored    bool
	}{
		{
			name: "store when nil",
			status: &v1alpha1.ApprovalTaskRunStatus{
				ApprovalTaskSpec: nil,
			},
			spec: &v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{Name: "user1", Type: "User"},
				},
			},
			shouldStore:    true,
			expectedStored: true,
		},
		{
			name: "don't store when already set",
			status: &v1alpha1.ApprovalTaskRunStatus{
				ApprovalTaskSpec: &v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{Name: "old-user", Type: "User"},
					},
				},
			},
			spec: &v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{Name: "new-user", Type: "User"},
				},
			},
			shouldStore:    false,
			expectedStored: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSpec := tt.status.ApprovalTaskSpec
			storeApprovalTaskSpec(tt.status, tt.spec)
			
			if tt.shouldStore {
				assert.NotNil(t, tt.status.ApprovalTaskSpec, "Spec should be stored")
				assert.Equal(t, tt.spec, tt.status.ApprovalTaskSpec, "Stored spec should match")
			} else {
				assert.Equal(t, originalSpec, tt.status.ApprovalTaskSpec, "Spec should not be overwritten")
			}
		})
	}
}

// TestCountApprovalsReceived tests the countApprovalsReceived function
func TestCountApprovalsReceived(t *testing.T) {
	tests := []struct {
		name           string
		approvalTask   v1alpha1.ApprovalTask
		expectedCount  int
	}{
		{
			name: "no approvals",
			approvalTask: v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{Name: "user1", Input: "pending", Type: "User"},
						{Name: "user2", Input: "pending", Type: "User"},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "one user approval",
			approvalTask: v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{Name: "user1", Input: "approve", Type: "User"},
						{Name: "user2", Input: "pending", Type: "User"},
					},
				},
			},
			expectedCount: 1,
		},
		{
			name: "two user approvals",
			approvalTask: v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{Name: "user1", Input: "approve", Type: "User"},
						{Name: "user2", Input: "approve", Type: "User"},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "group with approvals",
			approvalTask: v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{
							Name:  "dev-team",
							Input: "approve",
							Type:  "Group",
							Users: []v1alpha1.UserDetails{
								{Name: "alice", Input: "approve"},
								{Name: "bob", Input: "approve"},
								{Name: "charlie", Input: "pending"},
							},
						},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "mixed user and group approvals",
			approvalTask: v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{Name: "user1", Input: "approve", Type: "User"},
						{
							Name:  "dev-team",
							Input: "approve",
							Type:  "Group",
							Users: []v1alpha1.UserDetails{
								{Name: "alice", Input: "approve"},
								{Name: "bob", Input: "pending"},
							},
						},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "rejections not counted",
			approvalTask: v1alpha1.ApprovalTask{
				Spec: v1alpha1.ApprovalTaskSpec{
					Approvers: []v1alpha1.ApproverDetails{
						{Name: "user1", Input: "approve", Type: "User"},
						{Name: "user2", Input: "reject", Type: "User"},
					},
				},
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := countApprovalsReceived(tt.approvalTask)
			assert.Equal(t, tt.expectedCount, count, "Approval count should match")
		})
	}
}

// TestCompute tests the Compute hash function
func TestCompute(t *testing.T) {
	tests := []struct {
		name        string
		obj         interface{}
		expectError bool
		expectHash  bool
	}{
		{
			name: "valid object",
			obj: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expectError: false,
			expectHash:  true,
		},
		{
			name: "array",
			obj: []string{"item1", "item2"},
			expectError: false,
			expectHash:  true,
		},
		{
			name: "struct",
			obj: v1alpha1.ApproverDetails{
				Name: "user1",
				Type: "User",
			},
			expectError: false,
			expectHash:  true,
		},
		{
			name: "empty object",
			obj: map[string]interface{}{},
			expectError: false,
			expectHash:  true,
		},
		{
			name: "nil",
			obj: nil,
			expectError: false,
			expectHash:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := Compute(tt.obj)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectHash {
				assert.NotEmpty(t, hash, "Hash should not be empty")
				// SHA256 produces 64 character hex string
				assert.Len(t, hash, 64, "Hash should be 64 characters")
			}
		})
	}

	// Test that same object produces same hash
	obj1 := map[string]string{"key": "value"}
	hash1, err1 := Compute(obj1)
	assert.NoError(t, err1)

	obj2 := map[string]string{"key": "value"}
	hash2, err2 := Compute(obj2)
	assert.NoError(t, err2)

	assert.Equal(t, hash1, hash2, "Same object should produce same hash")

	// Test that different objects produce different hashes
	obj3 := map[string]string{"key": "different"}
	hash3, err3 := Compute(obj3)
	assert.NoError(t, err3)

	assert.NotEqual(t, hash1, hash3, "Different objects should produce different hashes")
}

// TestInitializeCustomRun tests the initializeCustomRun function
// Note: This test focuses on the initialization logic. The events.Emit call
// requires a Kubernetes event recorder which is not available in unit tests,
// but the core initialization logic (HasStarted, StartTime, InitializeConditions)
// is tested through integration tests in the reconciler.
func TestInitializeCustomRun(t *testing.T) {
	// This test is skipped because initializeCustomRun calls events.Emit
	// which requires a Kubernetes event recorder that's not available in unit tests.
	// The initialization logic is tested through the reconciler integration tests.
	// If you need to test this function directly, you would need to:
	// 1. Mock the events.Emit function, or
	// 2. Set up a proper test environment with a fake event recorder
	t.Skip("Skipping test because events.Emit requires Kubernetes event recorder")
}

// TestGetOrCreateApprovalTask tests the getOrCreateApprovalTask function
func TestGetOrCreateApprovalTask(t *testing.T) {
	ctx := context.Background()

	t.Run("get existing approval task", func(t *testing.T) {
		run := &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-run",
				Namespace: "test-ns",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: approvaltaskv1alpha1.SchemeGroupVersion.String(),
					Kind:       approvaltask.ControllerName,
				},
			},
		}

		// Create existing approval task
		client := fake.NewSimpleClientset()
		existingTask := &v1alpha1.ApprovalTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-run",
				Namespace: "test-ns",
			},
			Spec: v1alpha1.ApprovalTaskSpec{
				Approvers: []v1alpha1.ApproverDetails{
					{Name: "user1", Type: "User"},
				},
			},
		}
		_, err := client.OpenshiftpipelinesV1alpha1().ApprovalTasks("test-ns").Create(ctx, existingTask, metav1.CreateOptions{})
		assert.NoError(t, err)

		task, err := getOrCreateApprovalTask(ctx, client, run)
		assert.NoError(t, err)
		assert.NotNil(t, task)
		assert.Equal(t, "test-run", task.Name)
		assert.Equal(t, 1, len(task.Spec.Approvers))
	})

	t.Run("create new approval task when not exists", func(t *testing.T) {
		run := &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-run",
				Namespace: "test-ns",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: approvaltaskv1alpha1.SchemeGroupVersion.String(),
					Kind:       approvaltask.ControllerName,
				},
				Params: []v1beta1.Param{
					{
						Name:  "approvers",
						Value: *v1beta1.NewArrayOrString("user1"),
					},
				},
			},
		}

		client := fake.NewSimpleClientset()
		task, err := getOrCreateApprovalTask(ctx, client, run)
		assert.NoError(t, err)
		assert.NotNil(t, task)
		assert.Equal(t, "test-run", task.Name)
	})

	t.Run("handle custom spec", func(t *testing.T) {
		specJSON := `{"approvers":[{"name":"user1","type":"User"}]}`
		run := &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-run",
				Namespace: "test-ns",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: approvaltaskv1alpha1.SchemeGroupVersion.String(),
						Kind:       approvaltask.ControllerName,
					},
					Spec: runtime.RawExtension{
						Raw: []byte(specJSON),
					},
				},
			},
		}

		client := fake.NewSimpleClientset()
		task, err := getOrCreateApprovalTask(ctx, client, run)
		assert.NoError(t, err)
		assert.NotNil(t, task)
		assert.Equal(t, 1, len(task.Spec.Approvers))
	})
}

// TestCheckCustomRunReferencesApprovalTaskWithCustomSpec tests CustomSpec path
func TestCheckCustomRunReferencesApprovalTaskWithCustomSpec(t *testing.T) {
	t.Run("valid CustomSpec reference", func(t *testing.T) {
		run := &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: approvaltaskv1alpha1.SchemeGroupVersion.String(),
						Kind:       approvaltask.ControllerName,
					},
				},
			},
		}

		err := checkCustomRunReferencesApprovalTask(run)
		assert.NoError(t, err)
	})

	t.Run("invalid CustomSpec reference", func(t *testing.T) {
		run := &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "wrong-api",
						Kind:       "wrong-kind",
					},
				},
			},
		}

		err := checkCustomRunReferencesApprovalTask(run)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not reference a ApprovalTask custom CRD")
	})
}


