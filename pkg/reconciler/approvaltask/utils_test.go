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
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
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
