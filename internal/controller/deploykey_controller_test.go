/*
Copyright 2025.

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

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1alpha1 "github.com/odit-services/ghops/api/v1alpha1"
)

var _ = Describe("DeployKey Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		deploykey := &authv1alpha1.DeployKey{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind DeployKey")
			err := k8sClient.Get(ctx, typeNamespacedName, deploykey)
			if err != nil && errors.IsNotFound(err) {
				resource := &authv1alpha1.DeployKey{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: authv1alpha1.DeployKeySpec{
						Owner:      "odit-services",
						Repository: "ghops",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				// Mark the resource as already successful so reconcile stays local and
				// does not try to create external GitHub deploy keys in this unit test.
				resource.Status = authv1alpha1.DeployKeyStatus{
					Created: true,
					CrStatus: authv1alpha1.CrStatus{
						State:             authv1alpha1.StateSuccess,
						LastReconcileTime: time.Now().Format(time.RFC3339),
					},
				}
				Expect(k8sClient.Status().Update(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &authv1alpha1.DeployKey{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DeployKey")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DeployKeyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				logger: zap.NewNop().Sugar(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
