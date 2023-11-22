//go:build e2e
// +build e2e

package test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/handlers"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/handlers/app"
	"github.com/openshift-pipelines/manual-approval-gate/test/client"
	"github.com/openshift-pipelines/manual-approval-gate/test/resources"
	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime"
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

	t.Run("Update-the-approval-task", func(t *testing.T) {
		r := chi.NewRouter()
		r.Post("/approvaltask/{approvalTaskName}", func(w http.ResponseWriter, request *http.Request) {
			handlers.UpdateApprovalTask(w, request, clients.Dynamic)
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		data := `{"approved":"true", "namespace":"default"}`
		ep := "/approvaltask/" + cr.GetName()
		resp, err := http.Post(ts.URL+ep, "application/json", strings.NewReader(data))
		assert.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP status OK")

		var approvalTask *app.ApprovalTaskResult
		err = json.NewDecoder(resp.Body).Decode(&approvalTask)
		assert.NoError(t, err)

		assert.Equal(t, "true", string(approvalTask.Data.Approved))
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

	t.Run("Update-the-approval-task", func(t *testing.T) {
		r := chi.NewRouter()
		r.Post("/approvaltask/{approvalTaskName}", func(w http.ResponseWriter, request *http.Request) {
			handlers.UpdateApprovalTask(w, request, clients.Dynamic)
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		data := `{"approved":"false", "namespace":"default"}`
		ep := "/approvaltask/" + cr.GetName()
		resp, err := http.Post(ts.URL+ep, "application/json", strings.NewReader(data))
		assert.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP status OK")

		var approvalTask *app.ApprovalTaskResult
		err = json.NewDecoder(resp.Body).Decode(&approvalTask)
		assert.NoError(t, err)

		assert.Equal(t, "false", string(approvalTask.Data.Approved))
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
