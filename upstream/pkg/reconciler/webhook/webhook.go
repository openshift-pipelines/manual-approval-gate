package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

const (
	Group   = "openshift-pipelines.org"
	Version = "v1alpha1"
	Kind    = "ApprovalTask"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key  types.NamespacedName
	path string

	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	vwhlister    admissionlisters.ValidatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !r.IsLeaderFor(r.key) {
		return controller.NewSkipKey(key)
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := r.secretlister.Secrets(system.Namespace()).Get(r.secretName)
	if err != nil {
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}

	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", r.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return r.reconcileValidatingWebhook(ctx, caCert)
}

func (r *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if r.withContext != nil {
		ctx = r.withContext(ctx)
	}

	logger := logging.FromContext(ctx)
	kind := request.Kind

	newBytes := request.Object.Raw
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	if gvk.Group != Group || gvk.Version != Version || gvk.Kind != Kind {
		logger.Error("Unhandled kind: ", gvk)
	}

	// Decode new object 
	newObj, err := r.decodeNewObject(newBytes)
	if err != nil {
		return webhook.MakeErrorStatus("cannot decode incoming new object: %v", err)
	}

	// Validate structural requirements 
	if err := validateApprovalTask(newObj, ctx); err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	if request.Operation == "CREATE" {
		// For CREATE operations, ensure all approver inputs are set to "pending"
		if err := validateApproverInputsForCreate(newObj); err != nil {
			return webhook.MakeErrorStatus("validation failed: %v", err)
		}
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	if request.Operation != "UPDATE" {
		return webhook.MakeErrorStatus("unsupported operation: %s", request.Operation)
	}

	// Decode old object for UPDATE operations
	oldObj, err := r.decodeOldObject(request.OldObject.Raw)
	if err != nil {
		return webhook.MakeErrorStatus("cannot decode incoming old object: %v", err)
	}

	// Check if approval is required by the approver
	if !isApprovalRequired(*oldObj) {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "ApprovalTask has already reached it's final state",
			},
		}
	}

	// Check if username is mentioned in the approval task
	if !ifUserExists(oldObj.Spec.Approvers, request) {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "User does not exist in the approval list",
			},
		}
	}

	// Check if user is updating the input for his name only
	var userApprovalChanged bool
	errMsg := fmt.Errorf("User can only update their own approval input")

	// First check if user is trying to re-approve/re-reject their own already-decided task
	if alreadyDecidedMsg := checkIfUserAlreadyDecided(oldObj, newObj, request); alreadyDecidedMsg != "" {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: alreadyDecidedMsg,
			},
		}
	}

	changed, err := IsUserApprovalChanged(oldObj.Spec.Approvers, newObj.Spec.Approvers, request)
	if err != nil {
		userApprovalChanged = false
		errMsg = fmt.Errorf("Invalid input change: %v", err)
	} else if changed {
		if CheckOtherUsersForInvalidChanges(oldObj.Spec.Approvers, newObj.Spec.Approvers, request) {
			userApprovalChanged = true
		} else {
			userApprovalChanged = false
		}
	} else {
		userApprovalChanged = false
	}

	if !userApprovalChanged {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: errMsg.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func (ac *reconciler) reconcileValidatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)
	rules := []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"openshift-pipelines.org"},
				APIVersions: []string{"v1alpha1"},
				Resources:   []string{"approvaltask", "approvaltasks"},
			},
		},
	}

	configuredWebhook, err := ac.vwhlister.Get(ac.key.Name)
	if err != nil {
		return err
	}

	webhook := configuredWebhook.DeepCopy()

	webhook.OwnerReferences = nil

	for i, wh := range webhook.Webhooks {
		if wh.Name != webhook.Name {
			continue
		}
		webhook.Webhooks[i].Rules = rules
		webhook.Webhooks[i].ClientConfig.CABundle = caCert
		if webhook.Webhooks[i].ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		vwhclient := ac.client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
		if _, err := vwhclient.Update(ctx, webhook, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

func ifUserExists(approvals []v1alpha1.ApproverDetails, request *admissionv1.AdmissionRequest) bool {
	if len(approvals) == 0 {
		return true
	}
	for _, approval := range approvals {
		switch v1alpha1.DefaultedApproverType(approval.Type) {
		case "User":
			if approval.Name == request.UserInfo.Username {
				return true
			}
		case "Group":
			// Check if user is in the group by checking the group name against user's groups
			for _, userGroup := range request.UserInfo.Groups {
				if approval.Name == userGroup {
					return true
				}
			}
			// Also check if user is explicitly listed in the group's users
			for _, user := range approval.Users {
				if user.Name == request.UserInfo.Username {
					return true
				}
			}
		}
	}
	return false
}

func isApprovalRequired(approvaltask v1alpha1.ApprovalTask) bool {
	// If the task has reached a final state, no more approvals are needed
	if approvaltask.Status.State == "rejected" || approvaltask.Status.State == "approved" {
		return false
	}
	
	// Use the same logic as the controller to count approvals
	approvedUsers := make(map[string]bool)
	
	for _, approver := range approvaltask.Spec.Approvers {
		if approver.Input != "approve" {
			continue
		}
		
		if v1alpha1.DefaultedApproverType(approver.Type) == "User" {
			approvedUsers[approver.Name] = true
		} else if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			for _, user := range approver.Users {
				if user.Input == "approve" {
					approvedUsers[user.Name] = true
				}
			}
		}
	}
	
	// If we have enough approvals, the task should be approved (final state)
	if len(approvedUsers) >= approvaltask.Spec.NumberOfApprovalsRequired {
		return false
	}
	
	return true
}

// hasValidInputValue checks if the input value is either "approve" or "reject".
func hasValidInputValue(input string) error {
	if input == "approve" || input == "reject" {
		return nil
	}
	return fmt.Errorf("invalid input value: '%s'. Supported values are 'approve' or 'reject'", input)
}

// hasOnlyInputChanged checks if only the input field has changed for the current approver
// and if the new input value is valid
func hasOnlyInputChanged(oldObjApprover, newObjApprover v1alpha1.ApproverDetails) (bool, error) {
	if oldObjApprover.Name == newObjApprover.Name && oldObjApprover.Input != newObjApprover.Input {
		if err := hasValidInputValue(newObjApprover.Input); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// IsUserApprovalChanged checks if there is a valid input change for the current user.
func IsUserApprovalChanged(oldObjApprovers, newObjApprovers []v1alpha1.ApproverDetails, request *admissionv1.AdmissionRequest) (bool, error) {
	currentUser := request.UserInfo.Username
	for i, approver := range oldObjApprovers {
		if approver.Name == currentUser && v1alpha1.DefaultedApproverType(approver.Type) == "User" {
			return hasOnlyInputChanged(approver, newObjApprovers[i])
		}

		if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			// Check if current user is a member of this group
			isUserInGroup := false

			// Check if user is in the group by checking the group name against user's groups
			for _, userGroup := range request.UserInfo.Groups {
				if approver.Name == userGroup {
					isUserInGroup = true
					break
				}
			}

			// Also check if user is explicitly listed in the group's users
			for _, user := range approver.Users {
				if user.Name == currentUser {
					isUserInGroup = true
					break
				}
			}

			if isUserInGroup {
				// Allow changes to group-level input if user is in the group
				if i < len(newObjApprovers) {
					if approver.Input != newObjApprovers[i].Input {
						if err := hasValidInputValue(newObjApprovers[i].Input); err != nil {
							return false, err
						}
						return true, nil
					}
				}

				// Check if user is adding themselves to the group's users list
				oldUserFound := false
				newUserFound := false

				for _, user := range approver.Users {
					if user.Name == currentUser {
						oldUserFound = true
						break
					}
				}

				if i < len(newObjApprovers) {
					for _, user := range newObjApprovers[i].Users {
						if user.Name == currentUser {
							newUserFound = true
							break
						}
					}
				}

				// Allow user to add themselves to the group
				if !oldUserFound && newUserFound {
					// Validate the input they're setting for themselves
					if i < len(newObjApprovers) {
						for _, user := range newObjApprovers[i].Users {
							if user.Name == currentUser {
								if err := hasValidInputValue(user.Input); err != nil {
									return false, err
								}
								return true, nil
							}
						}
					}
					return true, nil
				}

				// Allow changes to individual user inputs within the group
				// Find current user in old users list
				var oldUserInput string
				userFoundInOld := false
				for _, user := range approver.Users {
					if user.Name == currentUser {
						oldUserInput = user.Input
						userFoundInOld = true
						break
					}
				}

				// Find current user in new users list
				var newUserInput string
				userFoundInNew := false
				if i < len(newObjApprovers) {
					for _, user := range newObjApprovers[i].Users {
						if user.Name == currentUser {
							newUserInput = user.Input
							userFoundInNew = true
							break
						}
					}
				}

				// Allow user to change their input if they're in both old and new lists
				if userFoundInOld && userFoundInNew && oldUserInput != newUserInput {
					if err := hasValidInputValue(newUserInput); err != nil {
						return false, err
					}
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// checkIfUserAlreadyDecided checks if a user is trying to re-approve/re-reject a task they've already decided on
func checkIfUserAlreadyDecided(oldObj *v1alpha1.ApprovalTask, newObj *v1alpha1.ApprovalTask, request *admissionv1.AdmissionRequest) string {
	currentUser := request.UserInfo.Username
	
	// Get user's desired new input from the incoming object
	desiredInput := ""
	
	// First check if user is an individual approver
	for _, approver := range newObj.Spec.Approvers {
		if v1alpha1.DefaultedApproverType(approver.Type) == "User" && approver.Name == currentUser {
			desiredInput = approver.Input
			break
		}
	}
	
	// If not found as individual user, check if user is in any group
	if desiredInput == "" {
		for _, approver := range newObj.Spec.Approvers {
			if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
				// Check if user is explicitly in the group's users list
				for _, user := range approver.Users {
					if user.Name == currentUser {
						desiredInput = user.Input
						break
					}
				}
				if desiredInput != "" {
					break
				}
				
				// Check if user is in the group via RBAC (group-level input)
				for _, userGroup := range request.UserInfo.Groups {
					if approver.Name == userGroup {
						desiredInput = approver.Input
						break
					}
				}
				if desiredInput != "" {
					break
				}
			}
		}
	}
	
	// Check status.approversResponse to see if user has already made a decision
	for _, approverResponse := range oldObj.Status.ApproversResponse {
		if v1alpha1.DefaultedApproverType(approverResponse.Type) == "User" && approverResponse.Name == currentUser {
			// Block duplicate approvals and any action after rejection
			if approverResponse.Response == "approved" && desiredInput == "approve" {
				return "User has already approved"
			} else if approverResponse.Response == "rejected" {
				return "User has already rejected"
			}
		}
		
		// Check if user is in any group that has responded
		if v1alpha1.DefaultedApproverType(approverResponse.Type) == "Group" {
			for _, member := range approverResponse.GroupMembers {
				if member.Name == currentUser {
					// Block duplicate approvals and any action after rejection
					if member.Response == "approved" && desiredInput == "approve" {
						return "User has already approved"
					} else if member.Response == "rejected" {
						return "User has already rejected"
					}
				}
			}
		}
	}
	
	return "" // No issue found
}

// CheckOtherUsersForInvalidChanges validates that no other approvers inputs have been changed
func CheckOtherUsersForInvalidChanges(oldObjApprovers, newObjApprover []v1alpha1.ApproverDetails, request *admissionv1.AdmissionRequest) bool {
	currentUser := request.UserInfo.Username
	for i, approver := range oldObjApprovers {
		if v1alpha1.DefaultedApproverType(approver.Type) == "User" && approver.Name != currentUser {
			if oldObjApprovers[i].Input != newObjApprover[i].Input {
				return false
			}
		}

		if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			// Check if current user is a member of this group
			isUserInGroup := false

			// Check if user is in the group by checking the group name against user's groups
			for _, userGroup := range request.UserInfo.Groups {
				if approver.Name == userGroup {
					isUserInGroup = true
					break
				}
			}

			// Also check if user is explicitly listed in the group's users
			for _, user := range approver.Users {
				if user.Name == currentUser {
					isUserInGroup = true
					break
				}
			}

			// If current user is not in this group, they shouldn't be able to change the group-level input
			if !isUserInGroup {
				if i < len(newObjApprover) && approver.Input != newObjApprover[i].Input {
					return false
				}
			}

			// Check that only current user's input has changed in group users
			// Build maps of existing users for easier comparison
			oldUsers := make(map[string]string) // name -> input
			newUsers := make(map[string]string) // name -> input

			for _, user := range approver.Users {
				oldUsers[user.Name] = user.Input
			}

			if i < len(newObjApprover) {
				for _, user := range newObjApprover[i].Users {
					newUsers[user.Name] = user.Input
				}
			}

			// Check that existing users (other than current user) haven't changed their input
			for userName, oldInput := range oldUsers {
				if userName != currentUser {
					if newInput, exists := newUsers[userName]; exists {
						if oldInput != newInput {
							return false // Someone else's input changed
						}
					}
				}
			}

			// Check that no unauthorized users were added to the group
			for userName := range newUsers {
				if _, existedBefore := oldUsers[userName]; !existedBefore {
					// Someone new was added - only allow if it's the current user and they're a group member
					if userName != currentUser {
						return false // Someone other than current user was added
					}
					if !isUserInGroup {
						return false // Current user is not a member of this group
					}
				}
			}
		}
	}

	return true
}

// validateApprovalTask validates the complete ApprovalTask resource 
func validateApprovalTask(approvalTask *v1alpha1.ApprovalTask, ctx context.Context) error {
	// Validate spec
	if err := validateApprovalTaskSpec(&approvalTask.Spec, ctx); err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}
	
	return nil
}

// validateApprovalTaskSpec validates the ApprovalTaskSpec
func validateApprovalTaskSpec(spec *v1alpha1.ApprovalTaskSpec, ctx context.Context) error {
	// Validate numberOfApprovalsRequired bounds
	if spec.NumberOfApprovalsRequired <= 0 {
		return fmt.Errorf("numberOfApprovalsRequired: must be greater than 0, got %d", spec.NumberOfApprovalsRequired)
	}

	// Validate approvers list
	if len(spec.Approvers) == 0 {
		return fmt.Errorf("approvers: required field is missing")
	}

	// Validate each approver and check for duplicates
	approverNames := make(map[string]int) // name -> index
	for i, approver := range spec.Approvers {
		fieldPath := fmt.Sprintf("approvers[%d]", i)
		
		if err := validateApprover(approver, fieldPath); err != nil {
			return err
		}
		
		// Check for duplicate approver names
		approverKey := fmt.Sprintf("%s:%s", v1alpha1.DefaultedApproverType(approver.Type), approver.Name)
		if existingIndex, exists := approverNames[approverKey]; exists {
			return fmt.Errorf("%s.name: duplicate approver '%s' (also found at approvers[%d])", fieldPath, approver.Name, existingIndex)
		}
		approverNames[approverKey] = i
	}

	return nil
}

// validateApprover validates a single approver entry
func validateApprover(approver v1alpha1.ApproverDetails, fieldPath string) error {
	// Validate approver type first to determine validation rules
	approverType := v1alpha1.DefaultedApproverType(approver.Type)
	if approverType != "User" && approverType != "Group" {
		return fmt.Errorf("%s.type: must be either 'User' or 'Group', got '%s'", fieldPath, approver.Type)
	}

	// Validate name format based on type (includes empty check via validateNameFormat)
	if approverType == "User" {
		if err := validateUserName(approver.Name); err != nil {
			return fmt.Errorf("%s.name: %w", fieldPath, err)
		}
	} else if approverType == "Group" {
		if err := validateGroupName(approver.Name); err != nil {
			return fmt.Errorf("%s.name: %w", fieldPath, err)
		}
	}

	// Validate input value
	validInputs := []string{"pending", "approve", "reject"}
	if !webhookContains(validInputs, approver.Input) {
		return fmt.Errorf("%s.input: must be one of: %s, got '%s'", fieldPath, strings.Join(validInputs, ", "), approver.Input)
	}

	// Validate users for group type
	if approverType == "Group" {
		
		// Track duplicate users within the group
		groupUsers := make(map[string]int) // username -> index
		for j, user := range approver.Users {
			userFieldPath := fmt.Sprintf("%s.users[%d]", fieldPath, j)
			
			if strings.TrimSpace(user.Name) == "" {
				return fmt.Errorf("%s.name: required field is missing", userFieldPath)
			} else if err := validateUserName(user.Name); err != nil {
				return fmt.Errorf("%s.name: %w", userFieldPath, err)
			}
			
			// Check for duplicate users within the group
			if existingIndex, exists := groupUsers[user.Name]; exists {
				return fmt.Errorf("%s.name: duplicate user '%s' within group (also found at %s.users[%d])", userFieldPath, user.Name, fieldPath, existingIndex)
			}
			groupUsers[user.Name] = j

			if !webhookContains(validInputs, user.Input) {
				return fmt.Errorf("%s.input: must be one of: %s, got '%s'", userFieldPath, strings.Join(validInputs, ", "), user.Input)
			}
		}
	}

	return nil
}

// validateNameFormat performs common name validation checks
func validateNameFormat(name, fieldType string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s cannot be empty", fieldType)
	}
	
	// Kubernetes names cannot contain spaces
	if strings.Contains(name, " ") {
		return fmt.Errorf("%s cannot contain spaces", fieldType)
	}
	
	return nil
}

// validateUserName validates username
func validateUserName(name string) error {
	// Basic empty check (spaces ARE allowed in usernames for LDAP integration)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("username cannot be empty")
	}
	
	if strings.HasPrefix(name, "group:") {
		return fmt.Errorf("username cannot start with 'group:' prefix - use type: Group for group approvers")
	}
	
	return nil
}

// validateGroupName validates group name format
func validateGroupName(name string) error {
	if err := validateNameFormat(name, "group name"); err != nil {
		return err
	}
	
	// Group names should not contain colons to avoid confusion with user prefixes
	if strings.Contains(name, ":") {
		return fmt.Errorf("group name cannot contain colons")
	}
	
	return nil
}

// webhookContains checks if a slice contains a string
func webhookContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// decodeNewObject decodes the incoming new object
func (r *reconciler) decodeNewObject(newBytes []byte) (*v1alpha1.ApprovalTask, error) {
	var newObj v1alpha1.ApprovalTask
	if len(newBytes) != 0 {
		newDecoder := json.NewDecoder(bytes.NewBuffer(newBytes))
		if r.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&newObj); err != nil {
			return nil, err
		}
	}
	return &newObj, nil
}

// decodeOldObject decodes the incoming old object
func (r *reconciler) decodeOldObject(oldBytes []byte) (*v1alpha1.ApprovalTask, error) {
	var oldObj v1alpha1.ApprovalTask
	if len(oldBytes) != 0 {
		oldDecoder := json.NewDecoder(bytes.NewBuffer(oldBytes))
		if r.disallowUnknownFields {
			oldDecoder.DisallowUnknownFields()
		}
		if err := oldDecoder.Decode(&oldObj); err != nil {
			return nil, err
		}
	}
	return &oldObj, nil
}

// validateApproverInputsForCreate ensures all approver inputs are set to "pending" for new ApprovalTask resources
func validateApproverInputsForCreate(approvalTask *v1alpha1.ApprovalTask) error {
	for i, approver := range approvalTask.Spec.Approvers {
		if approver.Input != "pending" {
			return fmt.Errorf("approvers[%d].input: must be 'pending' for new ApprovalTask, got '%s'", i, approver.Input)
		}
		
		// For group approvers, also validate that all users within the group have pending input
		if v1alpha1.DefaultedApproverType(approver.Type) == "Group" {
			for j, user := range approver.Users {
				if user.Input != "pending" {
					return fmt.Errorf("approvers[%d].users[%d].input: must be 'pending' for new ApprovalTask, got '%s'", i, j, user.Input)
				}
			}
		}
	}
	return nil
}
