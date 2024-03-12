package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key  types.NamespacedName
	path string

	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	mwhlister    admissionlisters.MutatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !ac.IsLeaderFor(ac.key) {
		return controller.NewSkipKey(key)
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}

	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileMutatingWebhook(ctx, caCert)
}

func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	logger := logging.FromContext(ctx)
	kind := request.Kind

	newBytes := request.Object.Raw
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	if gvk.Group != "openshift-pipelines.org" || gvk.Version != "v1alpha1" || gvk.Kind != "ApprovalTask" {
		logger.Error("Unhandled kind: ", gvk)
		fmt.Errorf("unhandled kind: %v", gvk)
	}

	oldBytes := request.OldObject.Raw
	var oldObj v1alpha1.ApprovalTask
	if len(newBytes) != 0 {
		newDecoder := json.NewDecoder(bytes.NewBuffer(oldBytes))
		if ac.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&oldObj); err != nil {
			return webhook.MakeErrorStatus("cannot decode incoming new object: %v", err)
		}
	}

	if !checkApprovalsRequired(oldObj) {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "ApprovalTask has already reached it's final state",
			},
		}
	}

	// Check if username is mentioned in the approval task
	if !checkIfUserExists(oldObj.Spec.Approvals, request.UserInfo.Username) {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "User does not exist in the in the approval list",
			},
		}
	}

	// Check if user is updating the input for his name only
	var newObj v1alpha1.ApprovalTask
	if len(newBytes) != 0 {
		newDecoder := json.NewDecoder(bytes.NewBuffer(newBytes))
		if ac.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&newObj); err != nil {
			return webhook.MakeErrorStatus("cannot decode incoming new object: %v", err)
		}
	}

	var userApprovalChanged bool

	for i, approval := range oldObj.Spec.Approvals {
		if approval.Name == request.UserInfo.Username {
			// Check if the corresponding approval in the new object is the only change.
			if newObj.Spec.Approvals[i].InputValue != approval.InputValue &&
				newObj.Spec.Approvals[i].Name == approval.Name {
				userApprovalChanged = true
			} else {
				// If there's any mismatch in other fields, consider it an invalid update.
				userApprovalChanged = false
				break
			}
		} else if newObj.Spec.Approvals[i].InputValue != approval.InputValue {
			// If any other user's input is changed, mark it as invalid.
			userApprovalChanged = false
			break
		}
	}

	if !userApprovalChanged {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "User can only update their own approval status",
			},
		}
	}

	if !checkIfMessageIsProvided(newObj, request.UserInfo.Username) {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "Please provide some message",
			},
		}
	}

	patches, err := roundTripPatch(newBytes, newObj)
	if err != nil {
		// TODO(puneet) fix the logger
		fmt.Println("Kuch toh gadbad hai daya", err)
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return webhook.MakeErrorStatus("error marshalling the data: %v", err)
	}

	// The `approvedBy` field is not added in the status
	return &admissionv1.AdmissionResponse{
		Patch:   patchBytes,
		Allowed: true,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ac *reconciler) reconcileMutatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)
	rules := []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Update,
				// admissionregistrationv1.Create,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"openshift-pipelines.org"},
				APIVersions: []string{"v1alpha1"},
				Resources:   []string{"approvaltask", "approvaltasks"},
			},
		},
	}

	configuredWebhook, err := ac.mwhlister.Get(ac.key.Name)
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
		mwhclient := ac.client.AdmissionregistrationV1().MutatingWebhookConfigurations()
		if _, err := mwhclient.Update(ctx, webhook, metav1.UpdateOptions{}); err != nil {
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

// roundTripPatch generates the JSONPatch that corresponds to round tripping the given bytes through
// the Golang type (JSON -> Golang type -> JSON). Because it is not always true that
// bytes == json.Marshal(json.Unmarshal(bytes)).
//
// For example, if bytes did not contain a 'spec' field and the Golang type specifies its 'spec'
// field without omitempty, then by round tripping through the Golang type, we would have added
// `'spec': {}`.
func roundTripPatch(bytes []byte, unmarshalled interface{}) (duck.JSONPatch, error) {
	if unmarshalled == nil {
		return duck.JSONPatch{}, nil
	}
	marshaledBytes, err := json.Marshal(unmarshalled)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal interface: %w", err)
	}
	return jsonpatch.CreatePatch(bytes, marshaledBytes)
}

func checkIfUserExists(approvals []v1alpha1.Input, currentUser string) bool {
	if len(approvals) == 0 {
		return true
	}
	for _, approval := range approvals {
		if approval.Name == currentUser {
			return true
		}
	}
	return false
}

func checkApprovalsRequired(approvaltask v1alpha1.ApprovalTask) bool {
	if approvaltask.Status.ApprovalState == "false" || approvaltask.Status.ApprovalState == "true" {
		return false
	}
	if len(approvaltask.Status.ApprovedBy) == approvaltask.Spec.ApprovalsRequired {
		return false
	}
	return true
}

func checkIfMessageIsProvided(approvaltask v1alpha1.ApprovalTask, username string) bool {
	for _, approval := range approvaltask.Spec.Approvals {
		if approval.Name == username {
			if approval.InputValue == "false" && approval.Message == "" {
				return false
			}
		}
	}
	return true
}

func returnErr(allowed bool, msg string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result: &metav1.Status{
			Message: msg,
		},
	}
}
