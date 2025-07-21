package test

import (
	"github.com/openshift-pipelines/manual-approval-gate/pkg/actions"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/cli"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Params struct {
	ns, kubeCfg, kubeCtx string
	ApprovalTask         versioned.Interface

	Kube k8s.Interface

	Cls      *cli.Clients
	Dynamic  dynamic.Interface
	Username string
	Groups   []string
}

func (p *Params) SetNamespace(ns string) {
	p.ns = ns
}
func (p *Params) Namespace() string {
	return p.ns
}

func (p *Params) SetNoColour(_ bool) {
}

func (p *Params) SetKubeConfigPath(path string) {
	p.kubeCfg = path
}

func (p *Params) SetKubeContext(context string) {
	p.kubeCtx = context
}

func (p *Params) KubeConfigPath() string {
	return p.kubeCfg
}

func (p *Params) GetUserInfo() (string, []string, error) {
	return p.Username, p.Groups, nil
}

func (p *Params) approvalTaskClient() (versioned.Interface, error) {
	return p.ApprovalTask, nil
}

func (p *Params) dynamicClient() (dynamic.Interface, error) {
	return p.Dynamic, nil
}

func (p *Params) KubeClient() (k8s.Interface, error) {
	return p.Kube, nil
}

func (p *Params) Clients(_ ...*rest.Config) (*cli.Clients, error) {
	if p.Cls != nil {
		return p.Cls, nil
	}

	approvaltask, err := p.approvalTaskClient()
	if err != nil {
		return nil, err
	}

	kube, err := p.KubeClient()
	if err != nil {
		return nil, err
	}

	dynamic, err := p.dynamicClient()
	if err != nil {
		return nil, err
	}

	if approvaltask != nil {
		if err := actions.InitializeAPIGroupRes(approvaltask.Discovery()); err != nil {
			return nil, err
		}
	}

	p.Cls = &cli.Clients{
		Kube:         kube,
		ApprovalTask: approvaltask,
		Dynamic:      dynamic,
	}

	return p.Cls, nil
}
