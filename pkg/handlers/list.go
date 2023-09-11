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

	"github.com/openshift-pipelines/manual-approval-gate/pkg/handlers/app"
	kErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func ListApprovalTask(res http.ResponseWriter, req *http.Request, dynamicClient dynamic.Interface) {

	// List custom resources by querying the API server.
	customResourceList, err := dynamicClient.Resource(app.CustomResourceGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if kErr.IsNotFound(err) {
			http.Error(res, "No resource found", http.StatusInternalServerError)
			return
		} else {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if len(customResourceList.Items) == 0 {
		http.Error(res, "No resource found", http.StatusInternalServerError)
		return
	}

	approvalTaskList := make([]app.ApprovalTask, 0)
	for _, cr := range customResourceList.Items {
		approved := cr.Object["spec"].(map[string]interface{})["approved"].(string)
		approvalTaskList = append(approvalTaskList, app.ApprovalTask{Name: cr.GetName(), Namespace: cr.GetNamespace(), Approved: approved})
	}

	approvalTask := app.ApprovalTaskList{
		Data: approvalTaskList,
	}

	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(approvalTask); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}
