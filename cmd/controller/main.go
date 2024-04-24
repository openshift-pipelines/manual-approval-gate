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
package main

import (
	"flag"
	"fmt"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/reconciler/approvaltask"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

var features = []string{"foo"}

const (
	// ControllerLogKey is the name of the logger for the controller cmd
	ControllerLogKey = "manual-approval-gate-controller"
)

func main() {
	fmt.Println(features)
	namespace := flag.String("namespace", corev1.NamespaceAll, "Namespace to restrict informer to. Optional, defaults to all namespaces.")

	// This parses flags.
	cfg := injection.ParseAndGetRESTConfigOrDie()

	ctx := injection.WithNamespaceScope(signals.NewContext(), *namespace)
	ctx = filteredinformerfactory.WithSelectors(ctx, v1alpha1.ManagedByLabelKey)
	sharedmain.MainWithConfig(ctx, ControllerLogKey, cfg,
		approvaltask.NewController(clock.RealClock{}),
	)
}
