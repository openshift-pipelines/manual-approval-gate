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

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/handlers/app"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestListApprovalTask(t *testing.T) {
	scheme := runtime.NewScheme()

	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "openshift-pipelines.org",
		Version: "v1alpha1",
		Kind:    "ApprovalTask",
	}, &unstructured.Unstructured{})

	// Create a fake client with the registered scheme and custom list kinds
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		app.CustomResourceGVR: "ApprovalTaskList",
	})

	// Create a fake custom resource and add it to the fake client.
	fakeApprovalTask := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "openshift-pipelines.org/v1alpha1",
			"kind":       "ApprovalTask",
			"metadata": map[string]interface{}{
				"name":      "example-task",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"approved": "true",
			},
		},
	}
	_, err := fakeClient.Resource(schema.GroupVersionResource{
		Group:    "openshift-pipelines.org",
		Version:  "v1alpha1",
		Resource: "approvaltasks",
	}).Namespace("default").Create(context.TODO(), fakeApprovalTask, metav1.CreateOptions{})
	assert.NoError(t, err, "Error creating fakeApprovalTask")

	req, err := http.NewRequest("GET", "/approvaltask", nil)
	assert.NoError(t, err)

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ListApprovalTask(w, r, fakeClient) // Pass the fakeClient to the handler
	})

	// Call the handler with the request and recorder.
	handler.ServeHTTP(recorder, req)
	assert.NoError(t, err)

	resp := recorder.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP status OK")

	var approvalTask *app.ApprovalTaskList
	err = json.NewDecoder(resp.Body).Decode(&approvalTask)
	assert.NoError(t, err)

	assert.Len(t, approvalTask.Data, 1, "Expected one ApprovalTask")
	assert.Equal(t, "example-task", approvalTask.Data[0].Name)
}

func TestListApprovalTaskNotFound(t *testing.T) {
	scheme := runtime.NewScheme()

	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "openshift-pipelines.org",
		Version: "v1alpha1",
		Kind:    "ApprovalTask",
	}, &unstructured.Unstructured{})

	// Create a fake client with the registered scheme and custom list kinds
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		app.CustomResourceGVR: "ApprovalTaskList",
	})

	req, err := http.NewRequest("GET", "/approvaltask", nil)
	assert.NoError(t, err)

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ListApprovalTask(w, r, fakeClient) // Pass the fakeClient to the handler
	})

	// Call the handler with the request and recorder.
	handler.ServeHTTP(recorder, req)
	assert.NoError(t, err)

	resp := recorder.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	bodyBytes, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "No resource found\n", string(bodyBytes))
}
