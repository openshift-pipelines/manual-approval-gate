package test

import (
	"fmt"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	fakeapprovaltaskclientset "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned/fake"
	informersv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/client/informers/externalversions/approvaltask/v1alpha1"
	fakeapprovaltaskclient "github.com/openshift-pipelines/manual-approval-gate/pkg/client/injection/client/fake"
	fakeapprovaltaskinformer "github.com/openshift-pipelines/manual-approval-gate/pkg/client/injection/informers/approvaltask/v1alpha1/approvaltask/fake"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
)

type Data struct {
	Approvaltasks []*v1alpha1.ApprovalTask
	Kube          *fakekubeclientset.Clientset
	Namespaces    []*corev1.Namespace
}

type Clients struct {
	ApprovalTask *fakeapprovaltaskclientset.Clientset
	Kube         *fakekubeclientset.Clientset
}

type Informers struct {
	ApprovalTask informersv1alpha1.ApprovalTaskInformer
}

// AddToInformer returns a function to add ktesting.Actions to the cache store
func AddToInformer(t *testing.T, store cache.Store) func(ktesting.Action) (bool, runtime.Object, error) {
	t.Helper()
	return func(action ktesting.Action) (bool, runtime.Object, error) {
		switch a := action.(type) {
		case ktesting.CreateActionImpl:
			if err := store.Add(a.GetObject()); err != nil {
				t.Fatal(err)
			}

		case ktesting.UpdateActionImpl:
			objMeta, err := meta.Accessor(a.GetObject())
			if err != nil {
				return true, nil, err
			}

			// Look up the old copy of this resource and perform the optimistic concurrency check.
			old, exists, err := store.GetByKey(objMeta.GetNamespace() + "/" + objMeta.GetName())
			if err != nil {
				return true, nil, err
			} else if !exists {
				// Let the client return the error.
				return false, nil, nil
			}
			oldMeta, err := meta.Accessor(old)
			if err != nil {
				return true, nil, err
			}
			// If the resource version is mismatched, then fail with a conflict.
			if oldMeta.GetResourceVersion() != objMeta.GetResourceVersion() {
				return true, nil, apierrs.NewConflict(
					a.Resource.GroupResource(), objMeta.GetName(),
					fmt.Errorf("resourceVersion mismatch, got: %v, wanted: %v",
						objMeta.GetResourceVersion(), oldMeta.GetResourceVersion()))
			}

			// Update the store with the new object when it's fine.
			if err := store.Update(a.GetObject()); err != nil {
				t.Fatal(err)
			}
		}
		return false, nil, nil
	}
}

func SeedTestData(t *testing.T, d Data) (Clients, Informers) {
	ctx, _ := ttesting.SetupFakeContext(t)
	t.Helper()
	c := Clients{
		ApprovalTask: fakeapprovaltaskclient.Get(ctx),
		Kube:         fakekubeclient.Get(ctx),
	}

	i := Informers{
		ApprovalTask: fakeapprovaltaskinformer.Get(ctx),
	}

	for _, n := range d.Namespaces {
		n := n.DeepCopy()
		if _, err := c.Kube.CoreV1().Namespaces().Create(ctx, n, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	c.ApprovalTask.PrependReactor("*", "approvaltask", AddToInformer(t, i.ApprovalTask.Informer().GetIndexer()))
	for _, at := range d.Approvaltasks {
		at := at.DeepCopy()
		if _, err := c.ApprovalTask.OpenshiftpipelinesV1alpha1().ApprovalTasks(at.Namespace).Create(ctx, at, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	c.ApprovalTask.ClearActions()
	c.Kube.ClearActions()
	return c, i
}
