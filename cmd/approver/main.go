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
	"log"
	"net/http"
	"time"

	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/signals"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openshift-pipelines/manual-approval-gate/pkg/handlers"
)

func main() {
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)

	r.Use(middleware.Timeout(60 * time.Second))

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx := signals.NewContext()
	ctx = injection.WithConfig(ctx, cfg)
	dynamicClient := dynamic.NewForConfigOrDie(cfg)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {})
	// FIXME use a real health check
	r.Get("/health", handlers.HealthCheck)
	// FIXME use a real readiness check
	r.Get("/readiness", handlers.HealthCheck)

	r.Get("/approvaltask", func(w http.ResponseWriter, r *http.Request) {
		handlers.ListApprovalTask(w, r, dynamicClient)
	})

	r.Post("/approvaltask/{approvalTaskName}", func(w http.ResponseWriter, r *http.Request) {
		handlers.UpdateApprovalTask(w, r, dynamicClient)
	})

	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":8000", r))
}
