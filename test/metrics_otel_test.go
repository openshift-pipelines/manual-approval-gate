//go:build e2e

// Copyright 2026 The Tekton Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	pkgTest "knative.dev/pkg/test"

	"github.com/openshift-pipelines/manual-approval-gate/test/utils"
)

const (
	magNamespace          = "tekton-pipelines"
	magMetricsPort        = "9090"
	magControllerSelector = "app.kubernetes.io/name=controller,app.kubernetes.io/part-of=openshift-pipelines-manual-approval-gates"
)

// magKubeClient builds a kubernetes client from the default kubeconfig.
func magKubeClient(t *testing.T) kubernetes.Interface {
	t.Helper()
	clients, err := utils.NewClients(pkgTest.Flags.Kubeconfig, pkgTest.Flags.Cluster, magNamespace)
	if err != nil {
		t.Fatalf("Failed to create clients: %v", err)
	}
	return clients.KubeClient
}

// scrapeMagControllerMetrics scrapes /metrics from the first Running/Ready
// manual-approval-gate controller pod via the Kubernetes API proxy.
// Returns an error so callers can retry on transient failures.
func scrapeMagControllerMetrics(ctx context.Context, kubeClient kubernetes.Interface) (map[string]*dto.MetricFamily, error) {
	pods, err := kubeClient.CoreV1().Pods(magNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: magControllerSelector,
	})
	if err != nil {
		return nil, err
	}

	var podName string
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		podReady := len(pod.Status.ContainerStatuses) > 0
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				podReady = false
				break
			}
		}
		if podReady {
			podName = pod.Name
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("no Running/Ready controller pod found for selector %q in namespace %s", magControllerSelector, magNamespace)
	}

	result := kubeClient.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName + ":" + magMetricsPort).
		Namespace(magNamespace).
		SubResource("proxy").
		Suffix("metrics").
		Do(ctx)

	body, err := result.Raw()
	if err != nil {
		return nil, err
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	return families, nil
}

// waitForMagMetric polls until the named metric appears in the controller metrics.
// Transient errors are logged and retried until timeout.
func waitForMagMetric(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, metricName string, timeout time.Duration) map[string]*dto.MetricFamily {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		families, err := scrapeMagControllerMetrics(ctx, kubeClient)
		if err == nil {
			if _, ok := families[metricName]; ok {
				return families
			}
		} else {
			t.Logf("Retrying metrics scrape: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for metric %q (waited %v): %v", metricName, timeout, ctx.Err())
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

// TestOTelMetricsController verifies that the manual-approval-gate controller
// exposes the expected OTel infrastructure metric families:
//   - http_client_* and kn_k8s_client_* (knative k8s client OTel instrumentation)
//   - kn_workqueue_* (knative reconciler workqueue)
//   - go_* runtime metrics
func TestOTelMetricsController(t *testing.T) {
	ctx := context.Background()
	kubeClient := magKubeClient(t)

	t.Log("Waiting for kn_workqueue_adds_total to appear on controller")
	families := waitForMagMetric(ctx, t, kubeClient, "kn_workqueue_adds_total", 2*time.Minute)
	t.Logf("Scraped %d metric families from manual-approval-gate controller", len(families))

	tests := []struct {
		name   string
		prefix string
		errMsg string
	}{
		{
			name:   "kn_workqueue_prefix",
			prefix: "kn_workqueue_",
			errMsg: "Expected at least one kn_workqueue_* metric, found none",
		},
		{
			name:   "http_client_prefix",
			prefix: "http_client_",
			errMsg: "Expected at least one http_client_* metric from knative k8s client instrumentation, found none",
		},
		{
			name:   "kn_k8s_client_prefix",
			prefix: "kn_k8s_client_",
			errMsg: "Expected at least one kn_k8s_client_* metric from knative k8s client instrumentation, found none",
		},
		{
			name:   "go_runtime_prefix",
			prefix: "go_",
			errMsg: "Expected standard go_* runtime metrics, found none",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name := range families {
				if strings.HasPrefix(name, tt.prefix) {
					return
				}
			}
			t.Error(tt.errMsg)
		})
	}
}
