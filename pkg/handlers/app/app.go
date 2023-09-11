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

package app

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Group    = "openshift-pipelines.org"
	Version  = "v1alpha1"
	Resource = "approvaltasks"
)

var (
	CustomResourceGVR = schema.GroupVersionResource{
		Group:    Group,
		Version:  Version,
		Resource: Resource,
	}
)

type ApprovalTask struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Approved  string `json:"approved"`
}

type ApprovalTaskList struct {
	Data []ApprovalTask `json:"data"`
}

type ApprovalTaskResult struct {
	Data ApprovalTask `json:"data"`
}
