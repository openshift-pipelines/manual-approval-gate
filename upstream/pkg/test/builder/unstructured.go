package builder

import (
	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func UnstructuredV1alpha1(task *v1alpha1.ApprovalTask, version string) *unstructured.Unstructured {
	task.APIVersion = "openshift-pipelines.org/" + version
	task.Kind = "approvaltask"
	object, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(task)
	return &unstructured.Unstructured{
		Object: object,
	}
}
