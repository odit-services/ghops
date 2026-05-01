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
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/google/go-github/v74/github"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	authv1alpha1 "github.com/odit-services/ghops/api/v1alpha1"
	"github.com/odit-services/ghops/internal/controller/mocks"
	"github.com/odit-services/ghops/internal/services"
)

const (
	testNamespace = "default"
	finalizerName = "auth.github.odit.services/deploykey"
	secretSuffix  = "-deploykey"
)

var _ = Describe("DeployKey Controller", func() {
	var (
		ctrl        *gomock.Controller
		mockGitHub  *mocks.MockGitHubClient
		mockSSH     *mocks.MockSSHService
		reconciler  *DeployKeyReconciler
		testCtx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockGitHub = mocks.NewMockGitHubClient(ctrl)
		mockSSH = mocks.NewMockSSHService(ctrl)
		testCtx = context.Background()
		reconciler = &DeployKeyReconciler{
			Client:     k8sClient,
			Scheme:     k8sClient.Scheme(),
			logger:     zap.NewNop().Sugar(),
			gitClient:  mockGitHub,
			sshservice: mockSSH,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		// Cleanup any created resources
		deploykeys := &authv1alpha1.DeployKeyList{}
		_ = k8sClient.List(testCtx, deploykeys, client.InNamespace(testNamespace))
		for _, dk := range deploykeys.Items {
			_ = k8sClient.Delete(testCtx, &dk)
			secretName := dk.Name + secretSuffix
			_ = k8sClient.Delete(testCtx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: testNamespace}})
		}
	})

	// Helper to create a DeployKey and return its object key
	createDeployKey := func(name string, spec authv1alpha1.DeployKeySpec, status *authv1alpha1.DeployKeyStatus, deletionTime *metav1.Time) client.ObjectKey {
		key := client.ObjectKey{Name: name, Namespace: testNamespace}
		dk := &authv1alpha1.DeployKey{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Namespace:         testNamespace,
				DeletionTimestamp: deletionTime,
			},
			Spec: spec,
		}
		if status != nil {
			dk.Status = *status
		}
		ExpectWithOffset(1, k8sClient.Create(testCtx, dk)).To(Succeed())
		
		if status != nil {
			// Get the latest resource and update status
			freshDK := &authv1alpha1.DeployKey{}
			ExpectWithOffset(1, k8sClient.Get(testCtx, key, freshDK)).To(Succeed())
			freshDK.Status = *status
			ExpectWithOffset(1, k8sClient.Status().Update(testCtx, freshDK)).To(Succeed())
		}
		return key
	}

	// Helper to get current DeployKey
	getDeployKey := func(key client.ObjectKey) *authv1alpha1.DeployKey {
		dk := &authv1alpha1.DeployKey{}
		ExpectWithOffset(1, k8sClient.Get(testCtx, key, dk)).To(Succeed())
		return dk
	}

	// Helper to check if secret exists
	secretExists := func(name, namespace string) bool {
		secret := &corev1.Secret{}
		err := k8sClient.Get(testCtx, types.NamespacedName{Name: name, Namespace: namespace}, secret)
		return err == nil
	}

	// Helper to run reconcile
	reconcileKey := func(key client.ObjectKey) (reconcile.Result, error) {
		return reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: key})
	}

	// Helper to make a GitHub Key response
	makeGitHubKey := func(id int64) *github.Key {
		return &github.Key{ID: github.Int64(id)}
	}

	// ==========================================================================
	// 1. Reconcile basics
	// ==========================================================================
	Describe("Reconcile basics", func() {
		Context("When resource does not exist", func() {
			It("should return nil error and empty result", func() {
				key := client.ObjectKey{Name: "nonexistent-deploykey", Namespace: testNamespace}
				result, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("When resource is in Failed state without DeletionTimestamp", func() {
			It("should skip reconciliation and return empty result", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					CrStatus: authv1alpha1.CrStatus{
						State: authv1alpha1.StateFailed,
					},
				}
				key := createDeployKey("failed-deploykey", spec, status, nil)

				result, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				dk := getDeployKey(key)
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateFailed))
			})
		})
	})

	// ==========================================================================
	// 2. Recently successful skip
	// ==========================================================================
	Describe("Recently successful skip", func() {
		Context("When resource was recently successful", func() {
			It("should return RequeueAfter without additional work", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created: true,
					CrStatus: authv1alpha1.CrStatus{
						State:             authv1alpha1.StateSuccess,
						LastReconcileTime: time.Now().Format(time.RFC3339),
					},
				}
				key := createDeployKey("recently-successful", spec, status, nil)

				result, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))

				dk := getDeployKey(key)
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateSuccess))
			})
		})
	})

	// ==========================================================================
	// 3. Create flow
	// ==========================================================================
	Describe("Create flow", func() {
		Context("When creating with ED25519 key type", func() {
			It("should generate keys, create secret, and register with GitHub", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
					Title:      "Test ED25519 Key",
				}
				key := createDeployKey("ed25519-create", spec, nil, nil)

				privKey := "-----BEGIN PRIVATE KEY-----\nED25519KEY\n-----END PRIVATE KEY-----"
				pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateED25519KeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(makeGitHubKey(12345), nil, nil),
				)

				result, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))

				dk := getDeployKey(key)
				Expect(dk.Status.Created).To(BeTrue())
				Expect(dk.Status.GitHubKeyID).To(Equal(int64(12345)))
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateSuccess))

				secretName := dk.Name + secretSuffix
				Expect(secretExists(secretName, testNamespace)).To(BeTrue())
			})
		})

		Context("When creating with RSA key type", func() {
			It("should generate RSA keys, create secret, and register with GitHub", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.RSA,
					Title:      "Test RSA Key",
				}
				key := createDeployKey("rsa-create", spec, nil, nil)

				privKey := "-----BEGIN RSA PRIVATE KEY-----\nRSAKEY\n-----END RSA PRIVATE KEY-----"
				pubKey := "ssh-rsa AAAAB3NzaC1yc2EAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateRSAKeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(makeGitHubKey(67890), nil, nil),
				)

				result, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))

				dk := getDeployKey(key)
				Expect(dk.Status.Created).To(BeTrue())
				Expect(dk.Status.GitHubKeyID).To(Equal(int64(67890)))
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateSuccess))
			})
		})

		// Note: "When key type is unknown" test removed - Kubernetes CRD validation
		// prevents creation of resources with invalid key types, so this scenario
		// can never be tested through the normal create flow. The validation
		// rejects such resources at the API level before they reach the controller.

		Context("When SSH key generation fails", func() {
			It("should call HandleError and status should not be Success", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
					Title:      "Failing Key",
				}
				key := createDeployKey("ssh-fail", spec, nil, nil)

				mockSSH.EXPECT().GenerateED25519KeyPair().Return("", "", errors.New("SSH generation failed"))

				_, _ = reconcileKey(key)

				dk := getDeployKey(key)
				Expect(dk.Status.State).NotTo(Equal(authv1alpha1.StateSuccess))
			})
		})

		Context("When GitHub API fails", func() {
			It("should call HandleError and status should not be Success", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
					Title:      "GitHub Fail Key",
				}
				key := createDeployKey("github-fail", spec, nil, nil)

				privKey := "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----"
				pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateED25519KeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(nil, nil, errors.New("GitHub API error")),
				)

				_, _ = reconcileKey(key)

				dk := getDeployKey(key)
				Expect(dk.Status.State).NotTo(Equal(authv1alpha1.StateSuccess))
			})
		})
	})

	// ==========================================================================
	// 4. Existing key validation
	// ==========================================================================
	Describe("Existing key validation", func() {
		Context("When key already exists with State=Reconciling", func() {
			It("should validate and update status to Success", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created:     true,
					GitHubKeyID: 12345,
					CrStatus: authv1alpha1.CrStatus{
						State: authv1alpha1.StateReconciling,
					},
				}
				key := createDeployKey("existing-key-reconciling", spec, status, nil)

				result, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())

				dk := getDeployKey(key)
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateSuccess))
				Expect(dk.Status.GitHubKeyID).To(Equal(int64(12345)))
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			})
		})

		Context("When key already exists with State=Pending", func() {
			It("should validate and update status to Success", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created:     true,
					GitHubKeyID: 99999,
					CrStatus: authv1alpha1.CrStatus{
						State: authv1alpha1.StatePending,
					},
				}
				key := createDeployKey("existing-key-pending", spec, status, nil)

				_, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())

				dk := getDeployKey(key)
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateSuccess))
			})
		})
	})

	// ==========================================================================
	// 5. Deletion flow
	// ==========================================================================
	Describe("Deletion flow", func() {
		Context("When DeletionTimestamp is set with no GitHubKeyID", func() {
			It("should skip GitHub deletion and remove finalizer", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created:     true,
					GitHubKeyID: 0,
					CrStatus: authv1alpha1.CrStatus{
						State: authv1alpha1.StateSuccess,
					},
				}
				// Create WITHOUT DeletionTimestamp
				key := createDeployKey("delete-no-github-id", spec, status, nil)

				// Ensure finalizer is set
				dk := getDeployKey(key)
				dk.Finalizers = []string{finalizerName}
				Expect(k8sClient.Update(testCtx, dk)).To(Succeed())

				// Trigger deletion via client.Delete (this sets DeletionTimestamp)
				dkToDelete := getDeployKey(key)
				Expect(k8sClient.Delete(testCtx, dkToDelete)).To(Succeed())

				// Verify DeletionTimestamp is now set
				dkAfterDelete := getDeployKey(key)
				Expect(dkAfterDelete.DeletionTimestamp).NotTo(BeNil())

				// GitHub DeleteKey should NOT be called (no GitHubKeyID)
				mockGitHub.EXPECT().DeleteKey(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				_, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())

				// After finalizer removal, the object may be garbage collected
				// Check by listing instead of direct Get
				dkList := &authv1alpha1.DeployKeyList{}
				err = k8sClient.List(testCtx, dkList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())

				for _, d := range dkList.Items {
					if d.Name == "delete-no-github-id" {
						// If still exists, verify finalizer removed and status updated
						Expect(d.Finalizers).NotTo(ContainElement(finalizerName))
						Expect(d.Status.State).To(Equal(authv1alpha1.StateSuccess))
					}
				}
			})
		})

		Context("When DeletionTimestamp is set with GitHubKeyID", func() {
			It("should delete from GitHub and remove finalizer", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created:     true,
					GitHubKeyID: 123,
					CrStatus: authv1alpha1.CrStatus{
						State: authv1alpha1.StateSuccess,
					},
				}
				// Create WITHOUT DeletionTimestamp
				key := createDeployKey("delete-with-github-id", spec, status, nil)

				// Ensure finalizer is set
				dk := getDeployKey(key)
				dk.Finalizers = []string{finalizerName}
				Expect(k8sClient.Update(testCtx, dk)).To(Succeed())

				// Trigger deletion via client.Delete (this sets DeletionTimestamp)
				dkToDelete := getDeployKey(key)
				Expect(k8sClient.Delete(testCtx, dkToDelete)).To(Succeed())

				mockGitHub.EXPECT().DeleteKey(testCtx, "test-owner", "test-repo", int64(123)).Return(nil, nil)

				_, err := reconcileKey(key)
				// Note: The object may be deleted after finalizer removal (expected behavior)
				// What matters is that the reconcile succeeded
				Expect(err).NotTo(HaveOccurred())
				
				// The object should either be deleted (no finalizer = garbage collected)
				// or have the finalizer removed. Check both cases.
				dkList := &authv1alpha1.DeployKeyList{}
				err = k8sClient.List(testCtx, dkList, client.InNamespace(testNamespace))
				Expect(err).NotTo(HaveOccurred())
				
				for _, d := range dkList.Items {
					if d.Name == "delete-with-github-id" {
						// If it still exists, finalizer should be removed
						Expect(d.Finalizers).NotTo(ContainElement(finalizerName))
					}
				}
				// Object may be deleted (0 items) or finalizer removed (if still exists)
			})
		})

		Context("When DeletionTimestamp is set with existing secret", func() {
			It("should delete the secret", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created:     true,
					GitHubKeyID: 0,
					CrStatus: authv1alpha1.CrStatus{
						State: authv1alpha1.StateSuccess,
					},
				}
				// Create WITHOUT DeletionTimestamp
				key := createDeployKey("delete-with-secret", spec, status, nil)

				// Create the secret first
				secretName := "delete-with-secret" + secretSuffix
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: testNamespace,
					},
					StringData: map[string]string{"identity": "test"},
				}
				Expect(k8sClient.Create(testCtx, secret)).To(Succeed())

				// Ensure finalizer is set
				dk := getDeployKey(key)
				dk.Finalizers = []string{finalizerName}
				Expect(k8sClient.Update(testCtx, dk)).To(Succeed())

				// Trigger deletion via client.Delete (this sets DeletionTimestamp)
				dkToDelete := getDeployKey(key)
				Expect(k8sClient.Delete(testCtx, dkToDelete)).To(Succeed())

				mockGitHub.EXPECT().DeleteKey(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				_, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())

				// Verify secret was deleted
				Expect(secretExists(secretName, testNamespace)).To(BeFalse())
			})
		})
	})

	// ==========================================================================
	// 6. Finalizer addition
	// ==========================================================================
	Describe("Finalizer addition", func() {
		Context("When resource has no finalizer", func() {
			It("should add finalizer and proceed with creation", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
				}
				key := createDeployKey("add-finalizer", spec, nil, nil)

				privKey := "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----"
				pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateED25519KeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(makeGitHubKey(111), nil, nil),
				)

				_, err := reconcileKey(key)
				Expect(err).NotTo(HaveOccurred())

				dk := getDeployKey(key)
				Expect(dk.Finalizers).To(ContainElement(finalizerName))
				Expect(dk.Status.Created).To(BeTrue())
			})
		})
	})

	// ==========================================================================
	// 7. HandleError retry/backoff
	// ==========================================================================
	Describe("HandleError retry/backoff", func() {
		Context("When GitHub API returns non-rate-limit error", func() {
			It("should increment retry count and return requeue with backoff", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
				}
				key := createDeployKey("retry-test", spec, nil, nil)

				privKey := "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----"
				pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateED25519KeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(nil, nil, errors.New("connection refused")),
				)

				result, err := reconcileKey(key)

				dk := getDeployKey(key)
				Expect(dk.Status.CurrentRetries).To(BeNumerically(">=", 1))
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
				_ = err
			})
		})

		Context("When GitHub API returns rate limit error", func() {
			It("should use longer backoff without incrementing retry count", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
				}
				key := createDeployKey("rate-limit-test", spec, nil, nil)

				privKey := "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----"
				pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateED25519KeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(nil, nil, errors.New("403 rate limit exceeded")),
				)

				result, err := reconcileKey(key)

				// Rate limit uses longer delay (RateLimitBaseDelay = 30 minutes)
				Expect(result.RequeueAfter).To(BeNumerically(">=", 30*time.Minute))
				_ = err
			})
		})

		Context("When max retries are exceeded", func() {
			It("should return error and set State to Failed", func() {
				spec := authv1alpha1.DeployKeySpec{
					Owner:      "test-owner",
					Repository: "test-repo",
					KeyType:    authv1alpha1.ED25519,
				}
				status := &authv1alpha1.DeployKeyStatus{
					Created: false,
					CrStatus: authv1alpha1.CrStatus{
						State:          authv1alpha1.StatePending,
						CurrentRetries: 4,
					},
				}
				key := createDeployKey("max-retries-test", spec, status, nil)

				privKey := "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----"
				pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAtest"

				gomock.InOrder(
					mockSSH.EXPECT().GenerateED25519KeyPair().Return(privKey, pubKey, nil),
					mockGitHub.EXPECT().CreateKey(testCtx, "test-owner", "test-repo", gomock.Any()).Return(nil, nil, errors.New("persistent error")),
				)

				_, err := reconcileKey(key)

				// When max retries reached, HandleError returns error
				Expect(err).To(HaveOccurred())

				dk := getDeployKey(key)
				Expect(dk.Status.State).To(Equal(authv1alpha1.StateFailed))
			})
		})
	})

	// ==========================================================================
	// 8. SSH Service tests
	// ==========================================================================
	Describe("SSH Service key generation", func() {
		Context("When using real SSH service", func() {
			It("should generate valid ED25519 keys", func() {
				sshService := &services.DefaultSSHService{RSAKeyLength: 2048}
				privKey, pubKey, err := sshService.GenerateED25519KeyPair()

				Expect(err).NotTo(HaveOccurred())
				Expect(privKey).NotTo(BeEmpty())
				Expect(pubKey).NotTo(BeEmpty())
				Expect(pubKey).To(HavePrefix("ssh-ed25519"))
			})

			It("should generate valid RSA keys", func() {
				sshService := &services.DefaultSSHService{RSAKeyLength: 2048}
				privKey, pubKey, err := sshService.GenerateRSAKeyPair()

				Expect(err).NotTo(HaveOccurred())
				Expect(privKey).NotTo(BeEmpty())
				Expect(pubKey).NotTo(BeEmpty())
				Expect(pubKey).To(HavePrefix("ssh-rsa"))
			})
		})
	})
})