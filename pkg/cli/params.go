package cli

import (
	"github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned/typed/approvaltask/v1alpha1"
	"github.com/pkg/errors"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Clients struct {
	Kube         k8s.Interface
	ApprovalTask approvaltaskv1alpha1.OpenshiftpipelinesV1alpha1Interface
}

type ApprovalTaskParams struct {
	clients        *Clients
	kubeConfigPath string
	kubeContext    string
	namespace      string
}

type Params interface {
	// SetKubeConfigPath uses the kubeconfig path to instantiate tekton
	// returned by Clientset function
	SetKubeConfigPath(string)
	// SetKubeContext extends the specificity of the above SetKubeConfigPath
	// by using a context other than the default context in the given kubeconfig
	SetKubeContext(string)
	SetNamespace(string)
	KubeClient() (k8s.Interface, error)
	Clients(...*rest.Config) (*Clients, error)
	Namespace() string
}

// ensure that TektonParams complies with cli.Params interface
var _ Params = (*ApprovalTaskParams)(nil)

func (p *ApprovalTaskParams) SetKubeConfigPath(path string) {
	p.kubeConfigPath = path
}

func (p *ApprovalTaskParams) SetKubeContext(context string) {
	p.kubeContext = context
}

func (p *ApprovalTaskParams) Namespace() string {
	return p.namespace
}

func (p *ApprovalTaskParams) SetNamespace(ns string) {
	p.namespace = ns
}

func (p *ApprovalTaskParams) kubeClient(config *rest.Config) (k8s.Interface, error) {
	k8scs, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create k8s client from config")
	}

	return k8scs, nil
}

func (p *ApprovalTaskParams) approvalTaskClient(config *rest.Config) (approvaltaskv1alpha1.OpenshiftpipelinesV1alpha1Interface, error) {
	approvalClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create dynamic client from config")

	}
	return approvalClient.OpenshiftpipelinesV1alpha1(), err
}

func (p *ApprovalTaskParams) config() (*rest.Config, error) {

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if p.kubeConfigPath != "" {
		loadingRules.ExplicitPath = p.kubeConfigPath
	}
	configOverrides := &clientcmd.ConfigOverrides{}
	if p.kubeContext != "" {
		configOverrides.CurrentContext = p.kubeContext
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	if p.namespace == "" {
		namespace, _, err := kubeConfig.Namespace()
		if err != nil {
			return nil, errors.Wrap(err, "Couldn't get kubeConfiguration namespace")
		}
		p.namespace = namespace
	}
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Parsing kubeconfig failed")
	}

	// set values as done in kubectl
	config.QPS = 50.0
	config.Burst = 300

	return config, nil
}

// Only returns kube client, not tekton client
func (p *ApprovalTaskParams) KubeClient() (k8s.Interface, error) {
	config, err := p.config()
	if err != nil {
		return nil, err
	}

	kube, err := p.kubeClient(config)
	if err != nil {
		return nil, err
	}

	return kube, nil
}

func (p *ApprovalTaskParams) Clients(cfg ...*rest.Config) (*Clients, error) {
	var config *rest.Config

	if len(cfg) != 0 && cfg[0] != nil {
		config = cfg[0]
	} else {
		defaultConfig, err := p.config()
		if err != nil {
			return nil, err
		}
		config = defaultConfig
	}

	kube, err := p.kubeClient(config)
	if err != nil {
		return nil, err
	}

	approvalClient, err := p.approvalTaskClient(config)
	if err != nil {
		return nil, err
	}

	p.clients = &Clients{
		Kube:         kube,
		ApprovalTask: approvalClient,
	}

	return p.clients, nil
}
