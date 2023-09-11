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

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	approvaltaskclient "github.com/openshift-pipelines/manual-approval-gate/pkg/client/injection/client"
	approvaltaskinformer "github.com/openshift-pipelines/manual-approval-gate/pkg/client/injection/informers/approvaltask/v1alpha1/approvaltask"
	pipelineclient "github.com/tektoncd/pipeline/pkg/client/injection/client"
	customruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	pipelinecontroller "github.com/tektoncd/pipeline/pkg/controller"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController instantiates a new controller.Impl from knative.dev/pkg/controller
func NewController() func(context.Context, configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {

		logger := logging.FromContext(ctx)
		kubeclientset := kubeclient.Get(ctx)
		pipelineclientset := pipelineclient.Get(ctx)
		approvaltaskclientset := approvaltaskclient.Get(ctx)
		customRunInformer := customruninformer.Get(ctx)
		approvaltaskInformer := approvaltaskinformer.Get(ctx)

		c := &Reconciler{
			kubeClientSet:         kubeclientset,
			pipelineClientSet:     pipelineclientset,
			approvaltaskClientSet: approvaltaskclientset,
			customRunLister:       customRunInformer.Lister(),
			approvaltaskLister:    approvaltaskInformer.Lister(),
		}

		impl := customrunreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
			return controller.Options{
				AgentName: "run-approvaltask",
			}
		})

		logger.Info("Setting up event handlers")

		// Add event handler for Runs
		customRunInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: pipelinecontroller.FilterCustomRunRef(approvaltaskv1alpha1.SchemeGroupVersion.String(), approvaltask.ControllerName),
			Handler:    controller.HandleAll(impl.Enqueue),
		})

		approvaltaskInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		return impl
	}
}
