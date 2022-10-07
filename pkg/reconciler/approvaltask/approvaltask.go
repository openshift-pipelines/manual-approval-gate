/*
Copyright 2022 The OpenShift Pipelines Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package approvaltask

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	approvaltaskclientset "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	listersapprovaltask "github.com/openshift-pipelines/manual-approval-gate/pkg/client/listers/approvaltask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	runreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1alpha1/run"
	listersalpha "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// approvaltaskLabelKey is the label identifier for a ApprovalTask.  This label is added to the Run and its TaskRuns.
	approvaltaskLabelKey = "/approvaltask"

	// approvaltaskRunLabelKey is the label identifier for a Run.  This label is added to the Run's TaskRuns.
	approvaltaskRunLabelKey = "/run"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	pipelineClientSet     clientset.Interface
	kubeClientSet         kubernetes.Interface
	approvaltaskClientSet approvaltaskclientset.Interface
	runLister             listersalpha.RunLister
	approvaltaskLister    listersapprovaltask.ApprovalTaskLister
	taskRunLister         listers.TaskRunLister
}

var (
	// Check that our Reconciler implements runreconciler.Interface
	_                runreconciler.Interface = (*Reconciler)(nil)
	cancelPatchBytes []byte
)

func init() {
	var err error
	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.TaskRunSpecStatusCancelled,
	}}
	cancelPatchBytes, err = json.Marshal(patches)
	if err != nil {
		log.Fatalf("failed to marshal patch bytes in order to cancel: %v", err)
	}
}

// ReconcileKind compares the actual state with the desired, and attempts to converge the two.
// It then updates the Status block of the Run resource with the current status of the resource.
func (c *Reconciler) ReconcileKind(ctx context.Context, run *v1alpha1.Run) pkgreconciler.Event {
	var merr error
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling Run %s/%s at %v", run.Namespace, run.Name, time.Now())

	// Check that the Run references a ApprovalTask CRD.  The logic is controller.go should ensure that only this type of Run
	// is reconciled this controller but it never hurts to do some bullet-proofing.
	var apiVersion, kind string
	if run.Spec.Ref != nil {
		apiVersion = run.Spec.Ref.APIVersion
		kind = string(run.Spec.Ref.Kind)
	} else if run.Spec.Spec != nil {
		apiVersion = run.Spec.Spec.APIVersion
		kind = run.Spec.Spec.Kind
	}
	if apiVersion != approvaltaskv1alpha1.SchemeGroupVersion.String() ||
		kind != approvaltask.ControllerName {
		logger.Errorf("Received control for a Run %s/%s that does not reference a ApprovalTask custom CRD", run.Namespace, run.Name)
		return nil
	}

	// If the Run has not started, initialize the Condition and set the start time.
	if !run.HasStarted() {
		logger.Infof("Starting new Run %s/%s", run.Namespace, run.Name)
		run.Status.InitializeConditions()
		// In case node time was not synchronized, when controller has been scheduled to other nodes.
		if run.Status.StartTime.Sub(run.CreationTimestamp.Time) < 0 {
			logger.Warnf("Run %s createTimestamp %s is after the Run started %s", run.Name, run.CreationTimestamp, run.Status.StartTime)
			run.Status.StartTime = &run.CreationTimestamp
		}
		// Emit events. During the first reconcile the status of the Run may change twice
		// from not Started to Started and then to Running, so we need to sent the event here
		// and at the end of 'Reconcile' again.
		// We also want to send the "Started" event as soon as possible for anyone who may be waiting
		// on the event to perform user facing initialisations, such has reset a CI check status
		afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(ctx, nil, afterCondition, run)
	}

	if run.IsDone() {
		logger.Infof("Run %s/%s is done", run.Namespace, run.Name)
		return nil
	}

	// Store the condition before reconcile
	beforeCondition := run.Status.GetCondition(apis.ConditionSucceeded)

	status := &approvaltaskv1alpha1.ApprovalTaskRunStatus{}
	if err := run.Status.DecodeExtraFields(status); err != nil {
		run.Status.MarkRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonInternalError.String(),
			"Internal error calling DecodeExtraFields: %v", err)
		logger.Errorf("DecodeExtraFields error: %v", err.Error())
	}

	// Reconcile the Run
	if err := c.reconcile(ctx, run, status); err != nil {
		logger.Errorf("Reconcile error: %v", err.Error())
		merr = multierror.Append(merr, err)
	}

	if err := c.updateLabelsAndAnnotations(ctx, run); err != nil {
		logger.Warn("Failed to update Run labels/annotations", zap.Error(err))
		merr = multierror.Append(merr, err)
	}

	if err := run.Status.EncodeExtraFields(status); err != nil {
		run.Status.MarkRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonInternalError.String(),
			"Internal error calling EncodeExtraFields: %v", err)
		logger.Errorf("EncodeExtraFields error: %v", err.Error())
	}

	afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
	events.Emit(ctx, beforeCondition, afterCondition, run)

	// Only transient errors that should retry the reconcile are returned.
	return merr
}

func (c *Reconciler) reconcile(ctx context.Context, run *v1alpha1.Run, status *approvaltaskv1alpha1.ApprovalTaskRunStatus) error {
	logger := logging.FromContext(ctx)

	// Get the ApprovalTask referenced by the Run
	approvaltaskMeta, approvaltaskSpec, err := c.getApprovalTask(ctx, logger, run)
	if err != nil {
		return err
	}

	// Store the fetched ApprovalTaskSpec on the Run for auditing
	storeApprovalTaskSpec(status, approvaltaskSpec)

	// Propagate labels and annotations from ApprovalTask to Run.
	propagateApprovalTaskLabelsAndAnnotations(run, approvaltaskMeta)

	// Validate ApprovalTask spec
	if err := approvaltaskSpec.Validate(ctx); err != nil {
		run.Status.MarkRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonFailedValidation.String(),
			"ApprovalTask %s/%s can't be Run; it has an invalid spec: %s",
			approvaltaskMeta.Namespace, approvaltaskMeta.Name, err)
		return nil
	}

	run.Status.MarkRunSucceeded(approvaltaskv1alpha1.ApprovalTaskRunReasonSucceeded.String(),
		"TaskRun succeeded")
	return nil

	return nil
}

func (c *Reconciler) getApprovalTask(ctx context.Context, logger *zap.SugaredLogger, run *v1alpha1.Run) (*metav1.ObjectMeta, *approvaltaskv1alpha1.ApprovalTaskSpec, error) {
	approvaltaskMeta := metav1.ObjectMeta{}
	approvaltaskSpec := approvaltaskv1alpha1.ApprovalTaskSpec{}
	if run.Spec.Ref != nil && run.Spec.Ref.Name != "" {
		// Use the k8 client to get the ApprovalTask rather than the lister.  This avoids a timing issue where
		// the ApprovalTask is not yet in the lister cache if it is created at nearly the same time as the Run.
		// See https://github.com/tektoncd/pipeline/issues/2740 for discussion on this issue.
		//
		tl, err := c.approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).Get(ctx, run.Spec.Ref.Name, metav1.GetOptions{})
		if err != nil {
			run.Status.MarkRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonCouldntGetApprovalTask.String(),
				"Error retrieving ApprovalTask for Run %s/%s: %s",
				run.Namespace, run.Name, err)
			return nil, nil, fmt.Errorf("Error retrieving ApprovalTask for Run %s: %w", fmt.Sprintf("%s/%s", run.Namespace, run.Name), err)
		}
		approvaltaskMeta = tl.ObjectMeta
		approvaltaskSpec = tl.Spec
	} else if run.Spec.Spec != nil {
		// FIXME(openshift-pipelines) support embedded spec
		if err := json.Unmarshal(run.Spec.Spec.Spec.Raw, &approvaltaskSpec); err != nil {
			run.Status.MarkRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonCouldntGetApprovalTask.String(),
				"Error retrieving ApprovalTask for Run %s/%s: %s",
				run.Namespace, run.Name, err)
			return nil, nil, fmt.Errorf("Error retrieving ApprovalTask for Run %s: %w", fmt.Sprintf("%s/%s", run.Namespace, run.Name), err)
		}
	}
	return &approvaltaskMeta, &approvaltaskSpec, nil
}

func propagateApprovalTaskLabelsAndAnnotations(run *v1alpha1.Run, approvaltaskMeta *metav1.ObjectMeta) {
	// Propagate labels from ApprovalTask to Run.
	if run.ObjectMeta.Labels == nil {
		run.ObjectMeta.Labels = make(map[string]string, len(approvaltaskMeta.Labels)+1)
	}
	for key, value := range approvaltaskMeta.Labels {
		run.ObjectMeta.Labels[key] = value
	}
	run.ObjectMeta.Labels[approvaltask.GroupName+approvaltaskLabelKey] = approvaltaskMeta.Name

	// Propagate annotations from ApprovalTask to Run.
	if run.ObjectMeta.Annotations == nil {
		run.ObjectMeta.Annotations = make(map[string]string, len(approvaltaskMeta.Annotations))
	}
	for key, value := range approvaltaskMeta.Annotations {
		run.ObjectMeta.Annotations[key] = value
	}
}

func storeApprovalTaskSpec(status *approvaltaskv1alpha1.ApprovalTaskRunStatus, tls *approvaltaskv1alpha1.ApprovalTaskSpec) {
	// Only store the ApprovalTaskSpec once, if it has never been set before.
	if status.ApprovalTaskSpec == nil {
		status.ApprovalTaskSpec = tls
	}
}
func (c *Reconciler) updateLabelsAndAnnotations(ctx context.Context, run *v1alpha1.Run) error {
	newRun, err := c.runLister.Runs(run.Namespace).Get(run.Name)
	if err != nil {
		return fmt.Errorf("error getting Run %s when updating labels/annotations: %w", run.Name, err)
	}
	if !reflect.DeepEqual(run.ObjectMeta.Labels, newRun.ObjectMeta.Labels) || !reflect.DeepEqual(run.ObjectMeta.Annotations, newRun.ObjectMeta.Annotations) {
		mergePatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels":      run.ObjectMeta.Labels,
				"annotations": run.ObjectMeta.Annotations,
			},
		}
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return err
		}
		_, err = c.pipelineClientSet.TektonV1alpha1().Runs(run.Namespace).Patch(ctx, run.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	}
	return nil
}
