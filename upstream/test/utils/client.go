package utils

import (
	manualApprovalVersioned "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned/typed/approvaltask/v1alpha1"
	pipelineVersioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Clients holds instances of interfaces for making requests to Tekton Pipelines.
type Clients struct {
	KubeClient         kubernetes.Interface
	Dynamic            dynamic.Interface
	ApprovalTaskClient approvaltaskv1alpha1.OpenshiftpipelinesV1alpha1Interface
	TektonClient       v1beta1.TektonV1beta1Interface
	Config             *rest.Config
	KubeClientSet      *kubernetes.Clientset
}

// NewClients instantiates and returns several clientsets required for making request to the
// TektonPipeline cluster specified by the combination of clusterName and configPath.
func NewClients(configPath string, clusterName string, namespace string) (*Clients, error) {
	clients := &Clients{}
	cfg, err := buildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	// We poll, so set our limits high.
	cfg.QPS = 100
	cfg.Burst = 200

	clients.KubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.KubeClientSet, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.TektonClient, err = newTektonBetaClients(cfg)
	if err != nil {
		return nil, err
	}

	clients.ApprovalTaskClient, err = newManualApprovalTaskAlphaClients(cfg)
	if err != nil {
		return nil, err
	}

	clients.Config = cfg
	return clients, nil
}

func buildClientConfig(kubeConfigPath string, clusterName string) (*rest.Config, error) {
	overrides := clientcmd.ConfigOverrides{}
	// Override the cluster name if provided.
	if clusterName != "" {
		overrides.Context.Cluster = clusterName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
		&overrides).ClientConfig()
}

func newTektonBetaClients(cfg *rest.Config) (v1beta1.TektonV1beta1Interface, error) {
	cs, err := pipelineVersioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cs.TektonV1beta1(), nil
}

func newManualApprovalTaskAlphaClients(cfg *rest.Config) (approvaltaskv1alpha1.OpenshiftpipelinesV1alpha1Interface, error) {
	cs, err := manualApprovalVersioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cs.OpenshiftpipelinesV1alpha1(), nil
}
