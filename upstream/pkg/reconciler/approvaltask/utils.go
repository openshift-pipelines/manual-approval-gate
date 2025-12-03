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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	v1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

var (
	gvk = schema.GroupVersionKind{Group: "tekton.dev", Version: "v1beta1", Kind: "CustomRun"}
)

// validateApproverParameter validates a single approver string (user or group format).
func validateApproverParameter(paramValue string, paramIndex int) error {
	if strings.TrimSpace(paramValue) == "" {
		return fmt.Errorf("approvers[%d]: approver name cannot be empty", paramIndex)
	}

	// Check for malformed group syntax
	if strings.Contains(paramValue, " :") || strings.Contains(paramValue, ": ") {
					return fmt.Errorf("approvers[%d]: invalid group format '%s' - use 'group:groupname' format (remove spaces around colon)", paramIndex, paramValue)
	}

	// Handle explicit group syntax: "group:groupname"
	if strings.HasPrefix(paramValue, "group:") {
		return validateGroupSyntax(paramValue, paramIndex)
	}

	return validateUserSyntax(paramValue, paramIndex)
}

// validateGroupSyntax validates the "group:groupname" format and ensures proper syntax.
func validateGroupSyntax(paramValue string, paramIndex int) error {
	parts := strings.SplitN(paramValue, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("approvers[%d]: invalid group format '%s' - group name cannot be empty after 'group:'", paramIndex, paramValue)
	}
	
	groupName := parts[1]
	// Validate group name format inline
	if strings.TrimSpace(groupName) == "" {
		return fmt.Errorf("approvers[%d]: group name cannot be empty", paramIndex)
	}
	if strings.Contains(groupName, ":") {
		return fmt.Errorf("approvers[%d]: group name '%s' cannot contain colons", paramIndex, groupName)
	}
	if strings.Contains(groupName, " ") {
		return fmt.Errorf("approvers[%d]: group name '%s' cannot contain spaces", paramIndex, groupName)
	}
	
	return nil
}

// validateUserSyntax validates a plain username approver.
func validateUserSyntax(paramValue string, paramIndex int) error {
	// Validate user name format inline
	if strings.TrimSpace(paramValue) == "" {
		return fmt.Errorf("approvers[%d]: username cannot be empty", paramIndex)
	}
	
	return nil
}

// ValidateCustomRunParameters validates CustomRun parameters for early error detection.
func ValidateCustomRunParameters(run *v1beta1.CustomRun) error {
	var hasApprovers bool
	var approversCount int
	var validationErrors []string

	for _, param := range run.Spec.Params {
		switch param.Name {
		case allApprovers:
			hasApprovers = true
			count, errs := validateApproversParam(param)
			approversCount = count
			validationErrors = append(validationErrors, errs...)
		case approvalsRequired:
			if err := validateApprovalsRequired(param.Value.StringVal); err != nil {
				return err
			}
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("invalid approvers parameter: %s", validationErrors[0])
	}

	if !hasApprovers {
		return fmt.Errorf("no valid approvers found - at least one approver is required")
	}

	if approversCount == 0 {
		return fmt.Errorf("no valid approvers found - at least one approver is required")
	}

	return nil
}

// validateApproversParam validates the approvers parameter and returns count + errors
func validateApproversParam(param v1beta1.Param) (int, []string) {
	var validationErrors []string
	var approversCount int

	// Parse approvers list from different formats
	approverList := parseApproversList(param, &validationErrors)

	// Validate each approver
	for i, approver := range approverList {
		switch val := approver.(type) {
		case string:
			if err := validateApproverParameter(val, i); err != nil {
				validationErrors = append(validationErrors, err.Error())
			} else {
				approversCount++
			}
		case map[string]interface{}:
			validateMalformedObjectApprover(val, i, &validationErrors)
		default:
			validationErrors = append(validationErrors, fmt.Sprintf("approvers[%d]: invalid approver format - must be a string", i))
		}
	}

	return approversCount, validationErrors
}

// parseApproversList extracts approvers from different parameter formats
func parseApproversList(param v1beta1.Param, validationErrors *[]string) []interface{} {
	var approverList []interface{}

	if len(param.Value.ArrayVal) > 0 {
		for _, approver := range param.Value.ArrayVal {
			approverList = append(approverList, approver)
		}
	}

	if len(param.Value.ObjectVal) > 0 {
		// Convert map[string]string to map[string]interface{} for proper handling
		objectVal := make(map[string]interface{})
		for k, v := range param.Value.ObjectVal {
			objectVal[k] = v
		}
		approverList = append(approverList, objectVal)
	}

	if param.Value.StringVal != "" && len(approverList) == 0 {
		var jsonData interface{}
		if err := json.Unmarshal([]byte(param.Value.StringVal), &jsonData); err != nil {
			*validationErrors = append(*validationErrors, fmt.Sprintf("failed to parse JSON '%s' - %v", param.Value.StringVal, err))
			return approverList
		}
		
		if arr, ok := jsonData.([]interface{}); ok {
			approverList = arr
		} else {
			*validationErrors = append(*validationErrors, "expected an array of approvers")
		}
	}

	return approverList
}

// validateMalformedObjectApprover validates malformed object approvers (from YAML with spaces)
func validateMalformedObjectApprover(approver map[string]interface{}, index int, validationErrors *[]string) {
	if groupName, ok := approver["group"]; ok {
		if groupStr, ok := groupName.(string); ok {
			// Format the object as JSON for clear error message
			objJSON := fmt.Sprintf(`{"group":"%s"}`, groupStr)
			*validationErrors = append(*validationErrors, fmt.Sprintf("approvers[%d]: invalid group format %s - use 'group:%s' format instead", index, objJSON, groupStr))
		} else {
			*validationErrors = append(*validationErrors, fmt.Sprintf("approvers[%d]: invalid group specification", index))
		}
	} else {
		// Handle other object formats
		objJSON, _ := json.Marshal(approver)
		*validationErrors = append(*validationErrors, fmt.Sprintf("approvers[%d]: invalid object format %s - approver must be a string, not an object", index, string(objJSON)))
	}
}

// validateApprovalsRequired validates the numberOfApprovalsRequired parameter value.
func validateApprovalsRequired(value string) error {
	approvals, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid numberOfApprovalsRequired parameter: '%s' is not a valid integer", value)
	}
	if approvals <= 0 {
		return fmt.Errorf("invalid numberOfApprovalsRequired parameter: must be greater than 0, got %d", approvals)
	}
	return nil
}

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

func getOrCreateApprovalTask(ctx context.Context, approvaltaskClientSet versioned.Interface, run *v1beta1.CustomRun) (*v1alpha1.ApprovalTask, error) {
	approvalTask := v1alpha1.ApprovalTask{}

	if run.Spec.CustomRef != nil {
		// Use the k8 client to get the ApprovalTask rather than the lister.  This avoids a timing issue where
		// the ApprovalTask is not yet in the lister cache if it is created at nearly the same time as the Run.
		// See https://github.com/tektoncd/pipeline/issues/2740 for discussion on this issue.
		tl, err := approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).Get(ctx, run.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				at, err := createApprovalTask(ctx, approvaltaskClientSet, run)
				if err != nil {
					return nil, err
				}
				return &at, nil
			}
		}
		approvalTask = *tl
	} else if run.Spec.CustomSpec != nil {
		// FIXME(openshift-pipelines) support embedded spec
		if err := json.Unmarshal(run.Spec.CustomSpec.Spec.Raw, &approvalTask.Spec); err != nil {
			run.Status.MarkCustomRunFailed(v1alpha1.ApprovalTaskRunReasonCouldntGetApprovalTask.String(),
				"Error retrieving ApprovalTask for Run %s/%s: %s",
				run.Namespace, run.Name, err)
			return nil, fmt.Errorf("Error retrieving ApprovalTask for Run %s: %w", fmt.Sprintf("%s/%s", run.Namespace, run.Name), err)
		}
	}

	return &approvalTask, nil
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

func createApprovalTask(ctx context.Context, approvaltaskClientSet versioned.Interface, run *v1beta1.CustomRun) (v1alpha1.ApprovalTask, error) {
	var (
		approvers      []v1alpha1.ApproverDetails
		users          []string
		desc           string
		err            error
		approverExists = make(map[string]bool)
		userExists     = make(map[string]bool)
	)

	logger := logging.FromContext(ctx)
	numberOfApprovalsRequired := 1

	for _, v := range run.Spec.Params {
		var approver v1alpha1.ApproverDetails

		if v.Name == allApprovers {
			for _, name := range v.Value.ArrayVal {
				if !userExists[name] {
					approver.Name = name
					approver.Input = pendingState

					// Check if the type is mentioned in the params
					if strings.HasPrefix(name, "group:") {
						approver.Type = "Group"

						if strings.HasPrefix(approver.Name, "group:") {
							parts := strings.SplitN(approver.Name, ":", 2)
							if len(parts) == 2 {
								approver.Name = parts[1]
							}
						}
					} else {
						approver.Type = "User"
					}

					if !approverExists[approver.Name] {
						approvers = append(approvers, approver)
						approverExists[approver.Name] = true
					}
					users = append(users, approver.Name)
					userExists[approver.Name] = true
				}
			}
		} else if v.Name == approvalsRequired {
			tempApproversRequired, err := strconv.Atoi(v.Value.StringVal)
			if err != nil {
				return v1alpha1.ApprovalTask{}, err
			}
			numberOfApprovalsRequired = tempApproversRequired
		} else if v.Name == description {
			desc = v.Value.StringVal
		}
	}

	ownerRef := *metav1.NewControllerRef(run, gvk)
	labels := make(map[string]string)
	for key, value := range run.Labels {
		labels[key] = value
	}
	labels[CustomRunLabelKey] = run.Name

	approvalTask := &v1alpha1.ApprovalTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:            run.Name,
			Namespace:       run.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.ApprovalTaskSpec{
			Approvers:                 approvers,
			NumberOfApprovalsRequired: numberOfApprovalsRequired,
			Description:               desc,
		},
	}

	approverSpecHash, err := Compute(approvalTask.Spec.Approvers)
	if err != nil {
		return v1alpha1.ApprovalTask{}, err
	}
	approvalTask.Annotations = map[string]string{
		LastAppliedHashKey: approverSpecHash,
	}

	_, err = approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).Create(ctx, approvalTask, metav1.CreateOptions{})
	if err != nil {
		return v1alpha1.ApprovalTask{}, err
	}
	logger.Infof("Approval Task %s is created", approvalTask.Name)

	at, err := approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).Get(ctx, run.Name, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("Error retrieving the created ApprovalTask %s: %v", run.Name, err)
		return v1alpha1.ApprovalTask{}, err
	}

	status := v1alpha1.ApprovalTaskStatus{
		State:             pendingState,
		Approvers:         users,
		ApproversResponse: []v1alpha1.ApproverState{},
		ApprovalsRequired: numberOfApprovalsRequired,
		ApprovalsReceived: 0, // Initially no approvals received
	}

	at.Status = status
	_, err = approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).UpdateStatus(ctx, at, metav1.UpdateOptions{})
	if err != nil {
		return v1alpha1.ApprovalTask{}, err
	}

	return *at, nil
}

func approvalTaskHasFalseInput(approvalTask v1alpha1.ApprovalTask) bool {
	for _, approver := range approvalTask.Spec.Approvers {
		if approver.Input == hasRejected {
			return true // Found an input that is "reject"
		}
	}
	return false
}

func approvalTaskHasTrueInput(approvalTask v1alpha1.ApprovalTask) bool {
	// Count approvers with input "approve"
	requiredApprovals := approvalTask.Spec.NumberOfApprovalsRequired

	approvedUsers := make(map[string]bool)

	for _, approver := range approvalTask.Spec.Approvers {
		if approver.Input != hasApproved {
			continue
		}

		if v1alpha1.DefaultedApproverType(approver.Type) == "User" {
			approvedUsers[approver.Name] = true
		} else if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			for _, user := range approver.Users {
				if user.Input == hasApproved {
					approvedUsers[user.Name] = true
				}
			}
		}
	}

	return len(approvedUsers) >= requiredApprovals
}

func countApprovalsReceived(approvalTask v1alpha1.ApprovalTask) int {
	// Count unique users who have approved
	approvedUsers := make(map[string]bool)

	for _, approver := range approvalTask.Spec.Approvers {
		if approver.Input != hasApproved {
			continue
		}

		if v1alpha1.DefaultedApproverType(approver.Type) == "User" {
			approvedUsers[approver.Name] = true
		} else if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			for _, user := range approver.Users {
				if user.Input == hasApproved {
					approvedUsers[user.Name] = true
				}
			}
		}
	}

	return len(approvedUsers)
}

func (r *Reconciler) checkIfUpdateRequired(ctx context.Context, approvalTask v1alpha1.ApprovalTask, run *v1beta1.CustomRun) error {
	logger := logging.FromContext(ctx)

	expectedHash, err := Compute(approvalTask.Spec.Approvers)
	if err != nil {
		logger.Errorf("Unable to compute the hash")
		return err
	}
	lastAppliedHash := approvalTask.GetAnnotations()[LastAppliedHashKey]

	if expectedHash != lastAppliedHash {
		if _, err := updateApprovalState(ctx, r.approvaltaskClientSet, &approvalTask); err != nil {
			return err
		}

		switch approvalTask.Status.State {
		case pendingState:
			logger.Infof("Approval task %s is in pending state", approvalTask.Name)
		case rejectedState:
			logger.Infof("Approval task %s is rejected", approvalTask.Name)
			run.Status.MarkCustomRunFailed(v1alpha1.ApprovalTaskRunReasonFailed.String(), "Approval Task denied")
		case approvedState:
			logger.Infof("Approval task %s is approved", approvalTask.Name)
			run.Status.MarkCustomRunSucceeded(v1alpha1.ApprovalTaskRunReasonSucceeded.String(),
				"TaskRun succeeded")
		}
	}

	return nil
}

func updateApprovalState(ctx context.Context, approvaltaskClientSet versioned.Interface, approvalTask *v1alpha1.ApprovalTask) (v1alpha1.ApprovalTask, error) {
	// Updating the approvedBy field in the status
	// Temp map to hold current approvers with approve and reject input
	currentApprovers := make(map[string]v1alpha1.ApproverState)
	approvalTask.Status.ApproversResponse = []v1alpha1.ApproverState{}
	// Track users who have already been processed as individual approvers
	// to avoid duplicate entries when they are also group members
	processedUserApprovers := make(map[string]bool)
	
	// First pass: Process all User type approvers
	for _, approver := range approvalTask.Spec.Approvers {
		if (approver.Input == hasApproved || approver.Input == hasRejected) && v1alpha1.DefaultedApproverType(approver.Type) == "User" {
			response := ""
			if approver.Input == hasApproved {
				response = approvedState
			} else if approver.Input == hasRejected {
				response = rejectedState
			}
			
			currentApprovers[approver.Name] = v1alpha1.ApproverState{
				Name:     approver.Name,
				Type:     "User",
				Response: response,
				Message:  approver.Message,
			}
			// Mark this user as processed to avoid duplication in group processing
			processedUserApprovers[approver.Name] = true
		}
	}
	
	// Second pass: Process Group type approvers, excluding users already processed as individuals
	for _, approver := range approvalTask.Spec.Approvers {
		if (approver.Input == hasApproved || approver.Input == hasRejected) && v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			groupMembers := []v1alpha1.GroupMemberState{}
			groupResponse := ""
			hasApprovals := false
			hasRejections := false

			for _, user := range approver.Users {
				// Skip users who have already been processed as individual approvers
				// This prevents duplicate entries when a user is both an individual approver and group member
				if processedUserApprovers[user.Name] {
					continue
				}
				
				userResponse := ""
				if user.Input == hasApproved {
					userResponse = approvedState
					hasApprovals = true
				} else if user.Input == hasRejected {
					userResponse = rejectedState
					hasRejections = true
				}

				if userResponse != "" {
					groupMembers = append(groupMembers, v1alpha1.GroupMemberState{
						Name:     user.Name,
						Response: userResponse,
						Message:  user.Message, // Inherit message from user level
					})
				}
			}

			// Determine group response based on individual user responses
			if hasRejections {
				groupResponse = rejectedState
			} else if hasApprovals {
				groupResponse = approvedState
			}

			if groupResponse != "" {
				currentApprovers[approver.Name] = v1alpha1.ApproverState{
					Name:         approver.Name,
					Type:         "Group",
					Response:     groupResponse,
					Message:      approver.Message,
					GroupMembers: groupMembers,
				}
			}
		}
	}

	if len(currentApprovers) != 0 {
		// Filter the ApprovedBy to only include those that are still true
		filteredApprovedBy := []v1alpha1.ApproverState{}
		for _, approver := range currentApprovers {
			filteredApprovedBy = append(filteredApprovedBy, approver)
		}

		// Update the ApprovedBy list
		approvalTask.Status.ApproversResponse = filteredApprovedBy

		// Update the approvals count fields
		approvalTask.Status.ApprovalsRequired = approvalTask.Spec.NumberOfApprovalsRequired
		approvalTask.Status.ApprovalsReceived = countApprovalsReceived(*approvalTask)

		// Update the approvalState
		// Reject scenario: Check if there is one false and if found mark the approvalstate to false
		// Approve scenario: Check if the input value from the user is true and is equal to the approvalsRequired
		if approvalTaskHasFalseInput(*approvalTask) {
			approvalTask.Status.State = rejectedState
		} else if approvalTaskHasTrueInput(*approvalTask) {
			approvalTask.Status.State = approvedState
		}

		// Update the status finally
		at, err := approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(approvalTask.Namespace).UpdateStatus(ctx, approvalTask, metav1.UpdateOptions{})
		if err != nil {
			return v1alpha1.ApprovalTask{}, err
		}
		return *at, nil
	}

	return v1alpha1.ApprovalTask{}, nil
}

// Compute generates an unique hash/string for the object pass to it.
// with sha256
func Compute(obj interface{}) (string, error) {
	d, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	hashSha256 := sha256.New()
	hashSha256.Write(d)
	return fmt.Sprintf("%x", hashSha256.Sum(nil)), nil
}
