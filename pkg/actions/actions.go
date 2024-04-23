package actions

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

var (
	doOnce      sync.Once
	apiGroupRes []*restmapper.APIGroupResources
)

// List fetches the resource and convert it to respective object
func List(gr schema.GroupVersionResource, c *cli.Clients, opts metav1.ListOptions, ns string, obj interface{}) error {
	unstructuredObj, err := list(gr, c.Dynamic, c.ApprovalTask.Discovery(), ns, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list objects from %s namespace \n", ns)
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), obj)
}

// list takes a partial resource and fetches a list of that resource's objects in the cluster using the dynamic client.
func list(gr schema.GroupVersionResource, dynamic dynamic.Interface, discovery discovery.DiscoveryInterface, ns string, op metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	gvr, err := GetGroupVersionResource(gr, discovery)
	if err != nil {
		return nil, err
	}

	allRes, err := dynamic.Resource(*gvr).Namespace(ns).List(context.Background(), op)
	if err != nil {
		return nil, err
	}

	return allRes, nil
}

func GetGroupVersionResource(gr schema.GroupVersionResource, discovery discovery.DiscoveryInterface) (*schema.GroupVersionResource, error) {
	var err error
	doOnce.Do(func() {
		err = InitializeAPIGroupRes(discovery)
	})
	if err != nil {
		return nil, err
	}

	rm := restmapper.NewDiscoveryRESTMapper(apiGroupRes)
	gvr, err := rm.ResourceFor(gr)
	if err != nil {
		return nil, err
	}

	return &gvr, nil
}

// InitializeAPIGroupRes initializes and populates the discovery client.
func InitializeAPIGroupRes(discovery discovery.DiscoveryInterface) error {
	var err error
	apiGroupRes, err = restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		return err
	}
	return nil
}
