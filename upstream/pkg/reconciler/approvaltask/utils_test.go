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
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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


