package dynamic

import (
	"github.com/openshift-pipelines/manual-approval-gate/pkg/test/dynamic/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func Client(objects ...runtime.Object) (dynamic.Interface, error) {
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Group: "openshift-pipelines.org", Version: "v1alpha1", Resource: "approvaltasks"}: "ApprovalTaskList",
		},
		objects...,
	)

	return clientset.New(clientset.WithClient(dynamicClient)), nil
}
