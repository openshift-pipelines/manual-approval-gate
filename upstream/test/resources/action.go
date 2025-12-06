package resources

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	manualApprovalVersioned "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	"github.com/openshift-pipelines/manual-approval-gate/test/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type Approver struct {
	Name  string
	Input string
}

func Create(t *testing.T, clients *utils.Clients, path string) *v1beta1.CustomRun {

	taskRunPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	taskRunYAML, err := ioutil.ReadFile(taskRunPath)
	if err != nil {
		t.Fatal(err)
	}

	customRun := MustParseCustomRun(t, string(taskRunYAML))

	var cr *v1beta1.CustomRun
	t.Run("ensure-custom-run-creation", func(t *testing.T) {
		cr, err = EnsureCustomTaskRunExists(clients.TektonClient, customRun)
		if err != nil {
			t.Fatalf("Failed to create the custom run: %v", err)
		}
	})

	t.Run("ensure-approval-task-creation", func(t *testing.T) {
		_, err := WaitForApprovalTaskCreation(clients.ApprovalTaskClient, cr.GetName(), cr.GetNamespace())
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}
	})

	return cr
}

func Update(t *testing.T, clients *utils.Clients, cr *v1beta1.CustomRun, approvers []Approver, state string) {
	t.Run("update the approval task", func(t *testing.T) {
		for _, approver := range approvers {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				at, err := clients.ApprovalTaskClient.ApprovalTasks(cr.GetNamespace()).Get(context.TODO(), cr.GetName(), metav1.GetOptions{})
				if err != nil {
					return err
				}

				att := updateApprovalTask(at, approver)

				clients.Config.Impersonate = rest.ImpersonationConfig{
					UserName: approver.Name,
				}

				clientSet, err := manualApprovalVersioned.NewForConfig(clients.Config)
				if err != nil {
					return fmt.Errorf("Failed to set the user: %v", err)
				}
				clients.ApprovalTaskClient = clientSet.OpenshiftpipelinesV1alpha1()

				_, err = clients.ApprovalTaskClient.ApprovalTasks(att.Namespace).Update(context.TODO(), att, metav1.UpdateOptions{})
				return err
			})

			if err != nil {
				t.Fatalf("Failed to update the approvalTask after retries: %v", err)
			}
		}

		_, err := WaitForApprovalTaskStatusUpdate(clients.ApprovalTaskClient, cr, state)
		if err != nil {
			t.Fatalf("Failed to update the approval task status: %v", err)
		}

		approvalTask, err := clients.ApprovalTaskClient.ApprovalTasks(cr.GetNamespace()).Get(context.TODO(), cr.GetName(), metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to get the approval task: %v", err)
		}
		assert.Equal(t, state, approvalTask.Status.State)
	})
}

func updateApprovalTask(at *v1alpha1.ApprovalTask, approver Approver) *v1alpha1.ApprovalTask {
	for i, a := range at.Spec.Approvers {
		if a.Name == approver.Name {
			at.Spec.Approvers[i].Input = approver.Input
		}
	}

	return at
}

func MustParseCustomRun(t *testing.T, yaml string) *v1beta1.CustomRun {
	t.Helper()
	var r v1beta1.CustomRun
	yaml = `apiVersion: tekton.dev/v1beta1
kind: CustomRun
` + yaml
	mustParseYAML(t, yaml, &r)
	return &r
}

func mustParseYAML(t *testing.T, yaml string, i runtime.Object) {
	t.Helper()
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, i); err != nil {
		t.Fatalf("mustParseYAML (%s): %v", yaml, err)
	}
}
