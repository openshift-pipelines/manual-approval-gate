/*
Copyright 2023 The OpenShift Pipelines Authors

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
	"reflect"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	v1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

func checkCustomRunReferencesApprovalTask(run *v1beta1.CustomRun) error {
	var apiVersion, kind string
	if run.Spec.CustomRef != nil {
		apiVersion = run.Spec.CustomRef.APIVersion
		kind = string(run.Spec.CustomRef.Kind)
	} else if run.Spec.CustomSpec != nil {
		apiVersion = run.Spec.CustomSpec.APIVersion
		kind = run.Spec.CustomSpec.Kind
	}

	if apiVersion != v1alpha1.SchemeGroupVersion.String() ||
		kind != approvaltask.ControllerName {
		return fmt.Errorf("Received control for a Run %s/%s that does not reference a ApprovalTask custom CRD", run.Namespace, run.Name)
	}
	return nil
}

func initializeCustomRun(ctx context.Context, run *v1beta1.CustomRun) {
	logger := logging.FromContext(ctx)
	if !run.HasStarted() {
		logger.Infof("Starting new Run %s/%s", run.Namespace, run.Name)
		run.Status.InitializeConditions()
		// In case node time was not synchronized, when controller has been scheduled to other nodes.
		if run.Status.StartTime.Sub(run.CreationTimestamp.Time) < 0 {
			logger.Warnf("Run %s createTimestamp %s is after the Run started %s", run.Name, run.CreationTimestamp, run.Status.StartTime)
			run.Status.StartTime = &run.CreationTimestamp
		}
		// Emit events. During the first reconcile the status of the Run may change twice
		// from not Started to Started and then to Running, so we need to send the event here
		// and at the end of 'Reconcile' again.
		// We also want to send the "Started" event as soon as possible for anyone who may be waiting
		// on the event to perform user facing initialisations, such as reset a CI check status
		afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
		events.Emit(ctx, nil, afterCondition, run)
	}
}

func (r *Reconciler) getOrCreateApprovalTask(ctx context.Context, run *v1beta1.CustomRun) (*metav1.ObjectMeta, *v1alpha1.ApprovalTaskSpec, error) {
	logger := logging.FromContext(ctx)
	approvalTaskMeta := metav1.ObjectMeta{}
	approvalTaskSpec := v1alpha1.ApprovalTaskSpec{}

	if run.Spec.CustomRef != nil {
		// Use the k8 client to get the ApprovalTask rather than the lister.  This avoids a timing issue where
		// the ApprovalTask is not yet in the lister cache if it is created at nearly the same time as the Run.
		// See https://github.com/tektoncd/pipeline/issues/2740 for discussion on this issue.
		tl, err := r.approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).Get(ctx, run.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				approvalTask := &v1alpha1.ApprovalTask{
					ObjectMeta: metav1.ObjectMeta{
						Name: run.Name,
					},
					Spec: v1alpha1.ApprovalTaskSpec{
						Name:     run.Name,
						Approved: "wait",
					},
				}

				tl, err = r.approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).Create(ctx, approvalTask, metav1.CreateOptions{})
				if err != nil {
					return nil, nil, err
				}
				logger.Infof("Approval Task %s is created", approvalTask.Name)
			}
		}
		approvalTaskMeta = tl.ObjectMeta
		approvalTaskSpec = tl.Spec
	} else if run.Spec.CustomSpec != nil {
		// FIXME(openshift-pipelines) support embedded spec
		if err := json.Unmarshal(run.Spec.CustomSpec.Spec.Raw, &approvalTaskSpec); err != nil {
			run.Status.MarkCustomRunFailed(v1alpha1.ApprovalTaskRunReasonCouldntGetApprovalTask.String(),
				"Error retrieving ApprovalTask for Run %s/%s: %s",
				run.Namespace, run.Name, err)
			return nil, nil, fmt.Errorf("Error retrieving ApprovalTask for Run %s: %w", fmt.Sprintf("%s/%s", run.Namespace, run.Name), err)
		}
	}
	return &approvalTaskMeta, &approvalTaskSpec, nil
}

func storeApprovalTaskSpec(status *v1alpha1.ApprovalTaskRunStatus, approvalTaskSpec *v1alpha1.ApprovalTaskSpec) {
	// Only store the ApprovalTaskSpec once, if it has never been set before.
	if status.ApprovalTaskSpec == nil {
		status.ApprovalTaskSpec = approvalTaskSpec
	}
}

func propagateApprovalTaskLabelsAndAnnotations(run *v1beta1.CustomRun, approvaltaskMeta *metav1.ObjectMeta) {
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

func (c *Reconciler) updateLabelsAndAnnotations(ctx context.Context, run *v1beta1.CustomRun) error {
	newRun, err := c.customRunLister.CustomRuns(run.Namespace).Get(run.Name)
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

		_, err = c.pipelineClientSet.TektonV1beta1().CustomRuns(run.Namespace).Patch(ctx, run.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	}
	return nil
}
