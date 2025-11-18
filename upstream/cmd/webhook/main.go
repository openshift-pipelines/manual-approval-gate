package main

import (
	"context"
	"os"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/reconciler/webhook"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	kwebhook "knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
)

func newValidationAdmissionController(name string) func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return webhook.NewAdmissionController(ctx,
			name,
			"/approval-validation",
			func(ctx context.Context) context.Context {
				return ctx
			},
			true,
		)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	serviceName := getEnvOrDefault("WEBHOOK_SERVICE_NAME", "manual-approval-webhook")
	secretName := getEnvOrDefault("WEBHOOK_SECRET_NAME", "manual-approval-gate-webhook-certs")
	webhookName := getEnvOrDefault("WEBHOOK_ADMISSION_CONTROLLER_NAME", "validation.webhook.manual-approval.openshift-pipelines.org")

	systemNamespace := os.Getenv("SYSTEM_NAMESPACE")
	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(signals.NewContext(), systemNamespace)

	// Set up a signal context with our webhook options
	ctx = kwebhook.WithOptions(ctx, kwebhook.Options{
		ServiceName: serviceName,
		Port:        kwebhook.PortFromEnv(8443),
		SecretName:  secretName,
	})

	sharedmain.WebhookMainWithConfig(ctx, serviceName,
		injection.ParseAndGetRESTConfigOrDie(),
		certificates.NewController,
		newValidationAdmissionController(webhookName),
	)
}
