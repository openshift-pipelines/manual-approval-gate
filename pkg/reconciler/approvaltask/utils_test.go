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
		assert.Equal(t, "pending", approver.Input, "Approval InputValue should be 'wait'")
	}

	assert.Equal(t, approvalTask.Status.State, "pending", "ApprovalState should be in `wait`")
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
				},
				{
					Name:  "bar",
					Input: "approve",
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
