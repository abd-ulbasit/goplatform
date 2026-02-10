/*
Copyright 2026 GoPlatform Authors.

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

// =============================================================================
// APPLICATION CONTROLLER TESTS
// =============================================================================
//
// These tests verify the controller's reconciliation logic using envtest.
//
// WHAT IS ENVTEST:
// ━━━━━━━━━━━━━━━━
//   Envtest runs a REAL Kubernetes API server and etcd (from test binaries),
//   giving us a genuine environment without needing a full cluster.
//
//   ┌─────────────────────────────────────────────────────────────────────────┐
//   │                         ENVTEST ARCHITECTURE                            │
//   │                                                                         │
//   │   ┌───────────────┐      ┌───────────────┐      ┌───────────────┐       │
//   │   │  Our Tests    │─────►│  kube-apiserver│─────►│    etcd       │       │
//   │   │               │      │  (from binaries)│     │  (from binaries)│     │
//   │   │  - Create CRDs│      │                │      │                │      │
//   │   │  - Reconcile  │      │  - Validates   │      │  - Stores      │      │
//   │   │  - Assert     │      │  - Serializes  │      │  - Watches     │      │
//   │   └───────────────┘      └───────────────┘      └───────────────┘       │
//   │                                                                         │
//   │   NO CONTROLLERS RUNNING BY DEFAULT                                     │
//   │   - We manually call Reconcile() to test it                             │
//   │   - This gives us deterministic, step-by-step testing                   │
//   │                                                                         │
//   └─────────────────────────────────────────────────────────────────────────┘
//
// WHY USE ENVTEST (vs alternatives):
//
//   ┌──────────────────────────────────────────────────────────────────────────┐
//   │ Approach          │ Pros                    │ Cons                       │
//   ├───────────────────┼─────────────────────────┼────────────────────────────┤
//   │ Unit tests with   │ Fast, isolated          │ Mocks can diverge from     │
//   │ mocks             │                         │ real K8s behavior          │
//   ├───────────────────┼─────────────────────────┼────────────────────────────┤
//   │ Envtest (we use)  │ Real API server,        │ Slower (~5s startup),      │
//   │                   │ catches schema bugs     │ no nodes/kubelet           │
//   ├───────────────────┼─────────────────────────┼────────────────────────────┤
//   │ kind/minikube     │ Full cluster, can test  │ Very slow, resource        │
//   │                   │ pod scheduling          │ intensive                  │
//   └──────────────────────────────────────────────────────────────────────────┘
//
// TESTING PATTERNS:
// ━━━━━━━━━━━━━━━━━
//   1. Create test resource
//   2. Manually call Reconcile()
//   3. Assert expected state (child resources created, status updated)
//   4. Repeat for edge cases (updates, deletes, errors)
//
// GINKGO/GOMEGA:
//   - BDD-style test framework (Describe/Context/It)
//   - Used by all Kubernetes projects
//   - Eventually() for async assertions
//
// =============================================================================

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	platformv1alpha1 "github.com/abd-ulbasit/goplatform/api/v1alpha1"
)

// =============================================================================
// TEST SUITE
// =============================================================================
//
// Each Describe block groups related tests.
// Context blocks provide different scenarios.
// It blocks are individual test cases.
//
// =============================================================================

var _ = Describe("Application Controller", func() {

	// =========================================================================
	// COMMON TEST FIXTURES
	// =========================================================================

	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond
	)

	// Helper to create a basic Application for testing
	createTestApplication := func(name, namespace string, withWorkload bool) *platformv1alpha1.Application {
		app := &platformv1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: platformv1alpha1.ApplicationSpec{
				Team:  "platform",
				Owner: "test@example.com",
				Tier:  platformv1alpha1.TierStandard,
			},
		}

		if withWorkload {
			replicas := int32(1)
			app.Spec.Workload = &platformv1alpha1.WorkloadSpec{
				Image:    "nginx:latest",
				Replicas: &replicas,
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				Ports: []platformv1alpha1.ContainerPort{
					{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
				},
				HealthCheck: &platformv1alpha1.HealthCheckSpec{
					Path:                "/health",
					Port:                8080,
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
					FailureThreshold:    3,
				},
			}
		}

		return app
	}

	// Helper to create reconciler
	// Uses a FakeRecorder to capture events during tests
	createReconciler := func() *ApplicationReconciler {
		return &ApplicationReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			// FakeRecorder with buffer of 100 events
			// In tests, we can check recorder.Events channel for emitted events
			Recorder: record.NewFakeRecorder(100),
		}
	}

	// =========================================================================
	// TEST: BASIC RECONCILIATION
	// =========================================================================
	//
	// Verifies that reconciling a basic Application:
	//   - Adds a finalizer
	//   - Creates a Deployment
	//   - Creates a Service
	//   - Updates status with conditions
	//
	// =========================================================================

	Context("When reconciling an Application with workload", func() {
		const resourceName = "test-app-workload"
		var typeNamespacedName types.NamespacedName

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			// Create the Application
			app := createTestApplication(resourceName, "default", true)
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			// Verify it was created
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, &platformv1alpha1.Application{})
			}, timeout, interval).Should(Succeed())
		})

		AfterEach(func() {
			// Clean up
			app := &platformv1alpha1.Application{}
			if err := k8sClient.Get(ctx, typeNamespacedName, app); err == nil {
				// Remove finalizer first to allow deletion
				app.Finalizers = nil
				_ = k8sClient.Update(ctx, app)
				_ = k8sClient.Delete(ctx, app)
			}

			// Clean up Deployment
			deploy := &appsv1.Deployment{}
			if err := k8sClient.Get(ctx, typeNamespacedName, deploy); err == nil {
				_ = k8sClient.Delete(ctx, deploy)
			}

			// Clean up Service
			svc := &corev1.Service{}
			if err := k8sClient.Get(ctx, typeNamespacedName, svc); err == nil {
				_ = k8sClient.Delete(ctx, svc)
			}
		})

		It("should add a finalizer on first reconcile", func() {
			// =====================================================================
			// TEST: FINALIZER ADDITION
			// =====================================================================
			//
			// The first reconcile should add our finalizer to ensure we can
			// clean up external resources before deletion.
			//
			// =====================================================================

			By("Reconciling the created resource")
			reconciler := createReconciler()
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue(), "should requeue after adding finalizer")

			By("Verifying the finalizer was added")
			app := &platformv1alpha1.Application{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
			Expect(app.Finalizers).To(ContainElement(applicationFinalizer))
		})

		It("should create a Deployment on reconcile", func() {
			// =====================================================================
			// TEST: DEPLOYMENT CREATION
			// =====================================================================
			//
			// After adding the finalizer, subsequent reconciles should create
			// the Deployment that runs the workload.
			//
			// =====================================================================

			By("Reconciling twice (first adds finalizer, second creates resources)")
			reconciler := createReconciler()

			// First reconcile - adds finalizer
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile - creates Deployment
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Deployment was created")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, deployment)
			}, timeout, interval).Should(Succeed())

			// Verify Deployment spec matches Application spec
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:latest"))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))

			// Verify labels
			Expect(deployment.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", resourceName))
			Expect(deployment.Labels).To(HaveKeyWithValue("platform.goplatform.io/team", "platform"))
			Expect(deployment.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "goplatform"))

			// Verify owner reference
			Expect(deployment.OwnerReferences).To(HaveLen(1))
			Expect(deployment.OwnerReferences[0].Kind).To(Equal("Application"))
		})

		It("should create a Service on reconcile", func() {
			// =====================================================================
			// TEST: SERVICE CREATION
			// =====================================================================
			//
			// A Service is created to provide stable networking
			// for the Deployment's pods.
			//
			// =====================================================================

			By("Reconciling twice")
			reconciler := createReconciler()

			// First reconcile - adds finalizer
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// Second reconcile - creates resources
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the Service was created")
			service := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, service)
			}, timeout, interval).Should(Succeed())

			// Verify Service spec
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Name).To(Equal("http"))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))

			// Verify selector matches Deployment pods
			Expect(service.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", resourceName))
		})

		It("should update Application status", func() {
			// =====================================================================
			// TEST: STATUS UPDATES
			// =====================================================================
			//
			// Status should reflect the current state of child resources.
			// Conditions provide granular status information.
			//
			// =====================================================================

			By("Reconciling the resource")
			reconciler := createReconciler()

			// First reconcile
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// Second reconcile
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			By("Verifying status was updated")
			app := &platformv1alpha1.Application{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, app); err != nil {
					return false
				}
				return len(app.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Verify conditions exist
			readyCondition := meta.FindStatusCondition(app.Status.Conditions, platformv1alpha1.ConditionTypeReady)
			Expect(readyCondition).NotTo(BeNil())

			workloadCondition := meta.FindStatusCondition(app.Status.Conditions, platformv1alpha1.ConditionTypeWorkloadReady)
			Expect(workloadCondition).NotTo(BeNil())

			// Verify observed generation
			Expect(app.Status.ObservedGeneration).To(Equal(app.Generation))
		})
	})

	// =========================================================================
	// TEST: APPLICATION WITHOUT WORKLOAD
	// =========================================================================
	//
	// Applications without a workload spec should:
	//   - Not create a Deployment or Service
	//   - Still update status correctly
	//   - Be marked Ready immediately
	//
	// USE CASE: Infrastructure-only applications (database, cache, etc.)
	//
	// =========================================================================

	Context("When reconciling an Application without workload", func() {
		const resourceName = "test-app-no-workload"
		var typeNamespacedName types.NamespacedName

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			// Create Application without workload
			app := createTestApplication(resourceName, "default", false)
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})

		AfterEach(func() {
			app := &platformv1alpha1.Application{}
			if err := k8sClient.Get(ctx, typeNamespacedName, app); err == nil {
				app.Finalizers = nil
				_ = k8sClient.Update(ctx, app)
				_ = k8sClient.Delete(ctx, app)
			}
		})

		It("should not create Deployment or Service", func() {
			By("Reconciling the resource")
			reconciler := createReconciler()

			// First reconcile - adds finalizer
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			// Second reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying no Deployment was created")
			deployment := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, typeNamespacedName, deployment)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			By("Verifying no Service was created")
			service := &corev1.Service{}
			err = k8sClient.Get(ctx, typeNamespacedName, service)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should mark as Ready", func() {
			By("Reconciling the resource")
			reconciler := createReconciler()

			// Reconcile until status is updated
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			By("Verifying Ready status")
			app := &platformv1alpha1.Application{}
			Eventually(func() platformv1alpha1.ApplicationPhase {
				if err := k8sClient.Get(ctx, typeNamespacedName, app); err != nil {
					return ""
				}
				return app.Status.Phase
			}, timeout, interval).Should(Equal(platformv1alpha1.ApplicationReady))
		})
	})

	// =========================================================================
	// TEST: RESOURCE GENERATION (MILESTONE 3)
	// =========================================================================

	Context("When generating specialized resources", func() {
		const resourceName = "test-app-resources"
		var typeNamespacedName types.NamespacedName

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}
		})

		AfterEach(func() {
			app := &platformv1alpha1.Application{}
			if err := k8sClient.Get(ctx, typeNamespacedName, app); err == nil {
				app.Finalizers = nil
				_ = k8sClient.Update(ctx, app)
				_ = k8sClient.Delete(ctx, app)
			}
		})

		It("should create ConfigMap, Secret, HPA, and PDB", func() {
			By("Creating the Application")
			app := createTestApplication(resourceName, "default", true)

			// Configure Scaling
			minReplicas := int32(2)
			app.Spec.Scaling = &platformv1alpha1.ScalingSpec{
				MinReplicas: &minReplicas,
				MaxReplicas: 5,
				Metrics: []platformv1alpha1.ScalingMetric{
					{Type: "cpu", Target: 80},
				},
			}

			// Configure High Availability (trigger PDB)
			app.Spec.Tier = platformv1alpha1.TierCritical

			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("Reconciling")
			reconciler := createReconciler()

			// Initial reconcile
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// Second reconcile to proceed past finalizers
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})

			By("Verifying ConfigMap")
			cm := &corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, cm)
			}, timeout, interval).Should(Succeed())
			Expect(cm.Data["APP_NAME"]).To(Equal(resourceName))

			By("Verifying Secret")
			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, secret)
			}, timeout, interval).Should(Succeed())

			By("Verifying HPA")
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, hpa)
			}, timeout, interval).Should(Succeed())
			Expect(hpa.Spec.MaxReplicas).To(Equal(int32(5)))
			Expect(*hpa.Spec.MinReplicas).To(Equal(int32(2)))

			By("Verifying PDB")
			pdb := &policyv1.PodDisruptionBudget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, pdb)
			}, timeout, interval).Should(Succeed())
			Expect(pdb.Spec.MinAvailable.IntVal).To(Equal(int32(1)))
		})
	})

	// =========================================================================
	// TEST: DELETION HANDLING
	// =========================================================================
	//
	// When an Application is deleted:
	//   1. deletionTimestamp is set
	//   2. Our finalizer blocks actual deletion
	//   3. handleDeletion() cleans up resources
	//   4. Finalizer is removed
	//   5. Kubernetes deletes the object
	//
	// =========================================================================

	Context("When deleting an Application", func() {
		const resourceName = "test-app-delete"
		var typeNamespacedName types.NamespacedName

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			// Create and reconcile Application
			app := createTestApplication(resourceName, "default", true)
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			reconciler := createReconciler()
			// Add finalizer
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// Create resources
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
		})

		AfterEach(func() {
			// Final cleanup if test didn't complete deletion
			app := &platformv1alpha1.Application{}
			if err := k8sClient.Get(ctx, typeNamespacedName, app); err == nil {
				app.Finalizers = nil
				_ = k8sClient.Update(ctx, app)
				_ = k8sClient.Delete(ctx, app)
			}
		})

		It("should remove finalizer and allow deletion", func() {
			By("Deleting the Application")
			app := &platformv1alpha1.Application{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())

			By("Reconciling to handle deletion (may require multiple reconciles)")
			reconciler := createReconciler()

			// The deletion handling now tracks deletion start time via annotation,
			// which requires a requeue after setting the annotation.
			// We reconcile multiple times until the finalizer is removed.
			for i := 0; i < 3; i++ {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				if err != nil || !result.Requeue {
					break
				}
			}

			By("Verifying the Application was deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, app)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	// =========================================================================
	// TEST: RECONCILING DELETED RESOURCE
	// =========================================================================
	//
	// If reconcile is called for a resource that no longer exists
	// (race condition, event coalescing), we should handle gracefully.
	//
	// =========================================================================

	Context("When reconciling a non-existent Application", func() {
		It("should return without error", func() {
			By("Reconciling a non-existent resource")
			reconciler := createReconciler()
			result, err := reconciler.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "does-not-exist",
					Namespace: "default",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})
})
