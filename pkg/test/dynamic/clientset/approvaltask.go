package clientset

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var allowedTektonTypes = map[string][]string{
	"v1alpha1": {"approvaltasks"},
}

func WithClient(client dynamic.Interface) Option {
	return func(cs *Clientset) {
		for version, resources := range allowedTektonTypes {
			for _, resource := range resources {
				r := schema.GroupVersionResource{
					Group:    "openshift-pipelines.org",
					Version:  version,
					Resource: resource,
				}
				cs.Add(r, client)
			}
		}
	}
}
