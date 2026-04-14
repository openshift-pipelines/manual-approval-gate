package builder

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func APIResourceList(version string, kinds []string) []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "openshift-pipelines.org" + "/" + version,
			APIResources: apiresources("openshift-pipelines.org", version, kinds),
		},
	}
}

func apiresources(group string, version string, kinds []string) []metav1.APIResource {
	apires := make([]metav1.APIResource, 0)
	for _, kind := range kinds {
		namespaced := true
		if strings.Contains(kind, "cluster") {
			namespaced = false
		}
		apires = append(apires, metav1.APIResource{
			Name:       kind + "s",
			Group:      group,
			Kind:       kind,
			Version:    version,
			Namespaced: namespaced,
		})
	}
	return apires
}
