package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	typedopenshiftpipelinesv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned/typed/approvaltask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	Interval = 10 * time.Second
	Timeout  = 10 * time.Minute
)

// EnsureTaskRunExists creates a TaskRun, if it does not exist.
func EnsureCustomTaskRunExists(client pipelinev1beta1.TektonV1beta1Interface, customRun *v1beta1.CustomRun) (*v1beta1.CustomRun, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	cr, err := client.CustomRuns(customRun.Namespace).Get(context.TODO(), customRun.Name, metav1.GetOptions{})
	if err != nil {
		cr, err = client.CustomRuns(customRun.Namespace).Create(context.TODO(), customRun, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}

	return cr, err
}

func WaitForApprovalTaskCreation(client typedopenshiftpipelinesv1alpha1.OpenshiftpipelinesV1alpha1Interface, name string) (*v1alpha1.ApprovalTask, error) {
	var lastState *v1alpha1.ApprovalTask
	waitErr := wait.PollImmediate(Interval, Timeout, func() (done bool, err error) {
		_, err = client.ApprovalTasks("default").Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("approval task %s is not in desired state", name)
	}

	return lastState, nil
}
