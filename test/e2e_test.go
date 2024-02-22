//go:build e2e
// +build e2e

package test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/openshift-pipelines/manual-approval-gate/test/client"
	"github.com/openshift-pipelines/manual-approval-gate/test/resources"
	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestApproveManualApprovalTask(t *testing.T) {
	clients := client.Setup(t, "default")

	taskRunPath, err := filepath.Abs("./testdata/customrun.yaml")
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
		cr, err = resources.EnsureCustomTaskRunExists(clients.TektonClient, customRun)
		if err != nil {
			t.Fatalf("Failed to create the custom run: %v", err)
		}
	})

	// Test if TektonConfig can reach the READY status
	t.Run("ensure-approval-task-creation", func(t *testing.T) {
		_, err := resources.WaitForApprovalTaskCreation(clients.ApprovalTaskClient, cr.GetName())
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}
	})

	t.Run("patch-the-approval-task", func(t *testing.T) {
		patchData := map[string]interface{}{
			"spec": map[string]interface{}{
				"approvers": []map[string]interface{}{
					{
						"input": "pending",
						"name":  "foo",
					},
					{
						"input": "pending",
						"name":  "bar",
					},
					{
						"input": "approve",
						"name":  "tekton",
					},
				},
			},
		}

		patch, err := json.Marshal(patchData)
		if err != nil {
			t.Fatal("Failed to update the approval task")
		}

		_, err = clients.ApprovalTaskClient.ApprovalTasks("default").Patch(context.TODO(), cr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			t.Fatal("Failed to patch the approval task", err)
		}

		patchData = map[string]interface{}{
			"spec": map[string]interface{}{
				"approvers": []map[string]interface{}{
					{
						"input": "pending",
						"name":  "foo",
					},
					{
						"input": "approve",
						"name":  "bar",
					},
					{
						"input": "approve",
						"name":  "tekton",
					},
				},
			},
		}

		patch, err = json.Marshal(patchData)
		if err != nil {
			t.Fatal("Failed to update the approval task")
		}

		_, err = clients.ApprovalTaskClient.ApprovalTasks("default").Patch(context.TODO(), cr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			t.Fatal("Failed to patch the approval task", err)
		}

		_, err = resources.WaitForApprovalTaskStatusUpdate(clients.ApprovalTaskClient, cr.GetName(), "approved")
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}

		approvalTask, err := clients.ApprovalTaskClient.ApprovalTasks("default").Get(context.TODO(), cr.GetName(), metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}
		assert.Equal(t, "approved", approvalTask.Status.State)
	})
}

func TestDisApproveManualApprovalTask(t *testing.T) {
	clients := client.Setup(t, "default")

	taskRunPath, err := filepath.Abs("./testdata/customrun.yaml")
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
		cr, err = resources.EnsureCustomTaskRunExists(clients.TektonClient, customRun)
		if err != nil {
			t.Fatalf("Failed to create the custom run: %v", err)
		}
	})

	// Test if TektonConfig can reach the READY status
	t.Run("ensure-approval-task-creation", func(t *testing.T) {
		_, err := resources.WaitForApprovalTaskCreation(clients.ApprovalTaskClient, cr.GetName())
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}
	})

	t.Run("patch-the-approval-task", func(t *testing.T) {
		patchData := map[string]interface{}{
			"spec": map[string]interface{}{
				"approvers": []map[string]interface{}{
					{
						"input": "pending",
						"name":  "foo",
					},
					{
						"input": "pending",
						"name":  "bar",
					},
					{
						"input": "reject",
						"name":  "tekton",
					},
				},
			},
		}

		patch, err := json.Marshal(patchData)
		if err != nil {
			t.Fatal("Failed to update the approval task")
		}

		_, err = clients.ApprovalTaskClient.ApprovalTasks("default").Patch(context.TODO(), cr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			t.Fatal("Failed to patch the approval task", err)
		}

		_, err = resources.WaitForApprovalTaskStatusUpdate(clients.ApprovalTaskClient, cr.GetName(), "rejected")
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}

		approvalTask, err := clients.ApprovalTaskClient.ApprovalTasks("default").Get(context.TODO(), cr.GetName(), metav1.GetOptions{})
		if err != nil {
			t.Fatal("Failed to get the approval task")
		}
		assert.Equal(t, "rejected", approvalTask.Status.State)
	})
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
