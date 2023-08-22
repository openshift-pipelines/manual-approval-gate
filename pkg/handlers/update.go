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
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/handlers/app"
	kErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

func UpdateApprovalTask(res http.ResponseWriter, req *http.Request, dynamicClient dynamic.Interface) {
	// Get the approvalTask Name from the url
	approvalTaskName := chi.URLParam(req, "approvalTaskName")

	var requestBody app.ApprovalTask
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&requestBody)
	if err != nil {
		http.Error(res, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Fetch the resource requested by the user
	customResource, err := dynamicClient.Resource(app.CustomResourceGVR).Namespace(requestBody.Namespace).Get(context.TODO(), approvalTaskName, metav1.GetOptions{})
	if err != nil {
		if kErr.IsNotFound(err) {
			http.Error(res, "No resource found", http.StatusNotFound)
			return
		} else {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"approved": requestBody.Approved,
		},
	}

	patch, err := json.Marshal(patchData)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// Patch the approvalTask using the data from the request body
	_, err = dynamicClient.Resource(app.CustomResourceGVR).Namespace(customResource.GetNamespace()).Patch(context.TODO(),
		customResource.GetName(),
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	at := &v1alpha1.ApprovalTask{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(customResource.Object, at)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	at.Spec.Approved = requestBody.Approved

	var approvalTaskStatus = &app.ApprovalTaskResult{
		Data: app.ApprovalTask{
			Name:     at.Name,
			Approved: at.Spec.Approved,
		},
	}

	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(approvalTaskStatus); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}
