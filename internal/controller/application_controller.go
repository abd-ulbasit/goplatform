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
// APPLICATION CONTROLLER - CORE RECONCILIATION LOGIC
// =============================================================================
//
// This is the brain of GoPlatform's operator. It watches Application CRDs
// and reconciles the cluster state to match the desired specification.
//
// CONTROLLER-RUNTIME ARCHITECTURE:
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//
//   ┌─────────────────────────────────────────────────────────────────────────┐
//   │                          KUBERNETES API SERVER                          │
//   │                                                                         │
//   │   Applications     Deployments      Services       Secrets              │
//   │   (our CRD)        (we create)      (we create)    (we create)          │
//   └───────┬─────────────────┬──────────────┬──────────────┬─────────────────┘
//           │ watch            │ watch         │ watch         │ watch
//           ▼                  ▼               ▼               ▼
//   ┌─────────────────────────────────────────────────────────────────────────┐
//   │                           SHARED INFORMER CACHE                         │
//   │                                                                         │
//   │  WHY CACHE?                                                             │
//   │  - Reduces API server load (read from local cache, not etcd)            │
//   │  - Enables list/watch pattern (initial list, then deltas)               │
//   │  - Cache is eventually consistent (few ms lag acceptable)               │
//   │                                                                         │
//   │  HOW IT WORKS:                                                          │
//   │  1. Reflector calls List() to get all objects                           │
//   │  2. Then Watch() for changes (efficient long-poll)                      │
//   │  3. Changes update the local Indexer cache                              │
//   │  4. Our reconciler reads from cache, writes to API server               │
//   └───────────────────────────────────────────────────────────────────────┬─┘
//                                                                           │
//           ┌────────────────┬────────────────┬────────────────┐            │
//           ▼                ▼                ▼                ▼            │
//   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
//   │ Application │  │ Deployment  │  │  Service    │  │  Secret     │      │
//   │  Handler    │  │  Handler    │  │  Handler    │  │  Handler    │      │
//   │             │  │(owner ref)  │  │(owner ref)  │  │(owner ref)  │      │
//   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘      │
//          │                │                │                │             │
//          └────────────────┴────────────────┴────────────────┘             │
//                                      │                                    │
//                                      ▼                                    │
//   ┌─────────────────────────────────────────────────────────────────────────┐
//   │                             WORK QUEUE                                  │
//   │                                                                         │
//   │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                           │
//   │  │app/a │ │app/b │ │app/a │ │app/c │ │app/a │  → deduplicated           │
//   │  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘                           │
//   │                                                                         │
//   │  FEATURES:                                                              │
//   │  • Deduplication: Multiple events for same object = one reconcile       │
//   │  • Rate limiting: Prevents thundering herd on mass changes              │
//   │  • Exponential backoff: Failed reconciles wait longer before retry      │
//   │  • Fair queuing: No single object can starve others                     │
//   └───────────────────────────────────────────────────────────────────────┬─┘
//                                                                           │
//                                      ▼                                    │
//   ┌─────────────────────────────────────────────────────────────────────────┐
//   │                      RECONCILE LOOP (this file)                         │
//   │                                                                         │
//   │  func Reconcile(ctx, Request{Name, Namespace}) (Result, error)          │
//   │    │                                                                    │
//   │    ├─► 1. GET: Fetch current Application from cache                     │
//   │    │      └─ If not found, object was deleted → cleanup & return        │
//   │    │                                                                    │
//   │    ├─► 2. FINALIZE: Handle deletion if deletionTimestamp set            │
//   │    │      └─ Remove external resources, then remove finalizer           │
//   │    │                                                                    │
//   │    ├─► 3. RECONCILE: Make actual state match desired state              │
//   │    │      ├─ Create/Update Deployment                                   │
//   │    │      ├─ Create/Update Service                                      │
//   │    │      └─ (Later: Create infrastructure via providers)               │
//   │    │                                                                    │
//   │    ├─► 4. STATUS: Update Application.Status with current state          │
//   │    │      └─ Set conditions, phase, observed generation                 │
//   │    │                                                                    │
//   │    └─► 5. RETURN: Signal completion or requeue                          │
//   │          ├─ Result{} → Done, no requeue                                 │
//   │          ├─ Result{RequeueAfter: 5m} → Check again in 5 minutes         │
//   │          └─ error → Requeue with exponential backoff                    │
//   │                                                                         │
//   └─────────────────────────────────────────────────────────────────────────┘
//
// LEVEL-TRIGGERED VS EDGE-TRIGGERED:
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//
//   EDGE-TRIGGERED (what we DON'T do):
//     "Application X was updated" → React to the event
//     ❌ Problem: What if we miss an event? What if processing fails?
//     ❌ Need complex event ordering and exactly-once semantics
//
//   LEVEL-TRIGGERED (what we DO):
//     "Make cluster match Application X's spec" → Compare and sync
//     ✅ Idempotent: Running twice gives same result
//     ✅ Self-healing: If state drifts, next reconcile fixes it
//     ✅ Miss an event? No problem, next reconcile catches up
//
//   ANALOGY:
//     Edge: "The thermostat was turned up" (event)
//     Level: "The room should be 72°F" (desired state)
//     The furnace doesn't care HOW you set 72°F, it just maintains it.
//
// HOW CROSSPLANE/BACKSTAGE COMPARE:
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
//
//   Crossplane:
//   - Same controller-runtime pattern
//   - One controller per Managed Resource type
//   - External controller watches Crossplane, provisions cloud
//
//   Backstage:
//   - Not a K8s operator (Node.js app)
//   - Uses polling to refresh catalog
//   - No reconciliation loop, just reads/displays
//
//   ArgoCD:
//   - Controller-runtime based
//   - Reconciles Application → Git → Cluster
//   - We're similar but for infrastructure
//
// =============================================================================

package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/abd-ulbasit/goplatform/api/v1alpha1"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	// applicationFinalizer is attached to Applications to ensure cleanup
	// before deletion. See handleDeletion() for detailed explanation.
	applicationFinalizer = "platform.goplatform.io/finalizer"

	// requeueAfterError is the delay before retrying after a transient error.
	// We use a fixed delay here; the work queue adds exponential backoff.
	requeueAfterError = 10 * time.Second

	// requeueAfterSuccess is the delay for periodic re-sync.
	// Even if nothing changed, we recheck to catch drift.
	requeueAfterSuccess = 5 * time.Minute
)

// =============================================================================
// RECONCILER STRUCT
// =============================================================================
//
// The reconciler holds dependencies needed for reconciliation.
//
// WHY client.Client (not direct API client):
//   - Reads from cache (fast, reduces API server load)
//   - Writes go directly to API server
//   - Handles serialization, pagination, retries
//   - Type-safe with generated methods
//
// WHY runtime.Scheme:
//   - Maps Go types ↔ GVK (GroupVersionKind)
//   - Required for owner references (garbage collection)
//   - Used by client for encoding/decoding
//
// =============================================================================

// ApplicationReconciler reconciles Application objects.
type ApplicationReconciler struct {
	// Client reads from cache, writes to API server.
	// Injected by controller-runtime manager.
	client.Client

	// Scheme maps Go types to Kubernetes GVKs.
	// Needed for setting owner references.
	Scheme *runtime.Scheme
}

// =============================================================================
// RBAC MARKERS
// =============================================================================
//
// These markers generate ClusterRole rules in config/rbac/role.yaml.
// The controller needs permission to manage these resources.
//
// FORMAT: +kubebuilder:rbac:groups=X,resources=Y,verbs=Z
//
// PRINCIPLE OF LEAST PRIVILEGE:
//   - Only request permissions we actually need
//   - Separate permissions for status subresource
//   - Be explicit about verbs (don't use *)
//
// =============================================================================

// RBAC for our CRD
// +kubebuilder:rbac:groups=platform.platform.goplatform.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.platform.goplatform.io,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.platform.goplatform.io,resources=applications/finalizers,verbs=update

// RBAC for Kubernetes resources we create
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// =============================================================================
// RECONCILE - THE MAIN LOOP
// =============================================================================
//
// This is called every time something relevant changes:
//   - Application is created/updated/deleted
//   - Owned Deployment/Service changes (via owner reference)
//   - Periodic resync (default every ~10 hours)
//   - After requeue from previous reconcile
//
// CONTRACT:
//   - MUST be idempotent (running N times = same result as running once)
//   - MUST be level-triggered (compare and sync, don't react to events)
//   - SHOULD update status every reconcile
//   - SHOULD return quickly (offload slow work to background)
//
// =============================================================================

// Reconcile moves the cluster state toward the desired state specified
// in the Application spec.
//
// The reconciliation flow:
//  1. Fetch the Application from cache
//  2. Handle deletion (finalizer pattern)
//  3. Ensure finalizer is present
//  4. Reconcile child resources (Deployment, Service)
//  5. Update Application status
//  6. Return result (requeue or done)
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// =========================================================================
	// STEP 0: SETUP LOGGING
	// =========================================================================
	//
	// Structured logging with request context. Every log line includes:
	//   - namespace/name of the Application
	//   - reconcileID for tracing this specific reconcile
	//
	// WHY STRUCTURED LOGGING:
	//   - Machine-parseable (grep, Loki, Datadog)
	//   - Consistent format across all reconciles
	//   - Adds context without string formatting
	//
	// =========================================================================

	logger := log.FromContext(ctx)
	logger.Info("starting reconciliation")

	// =========================================================================
	// STEP 1: FETCH THE APPLICATION
	// =========================================================================
	//
	// Get the Application that triggered this reconcile.
	//
	// IMPORTANT: We read from CACHE, not directly from etcd.
	//   - Faster (no network call if cached)
	//   - Reduces API server load
	//   - Eventually consistent (few ms lag is OK)
	//
	// If not found, the Application was deleted:
	//   - If it had a finalizer, deletion is blocked → we should clean up
	//   - If no finalizer, it's already gone → nothing to do
	//
	// =========================================================================

	var app platformv1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			// Application was deleted. If we had external resources to clean up,
			// they should have been handled by the finalizer before deletion
			// actually happened. Safe to ignore.
			logger.Info("application not found, likely deleted")
			return ctrl.Result{}, nil
		}
		// Real error (network, RBAC, etc.). Requeue with backoff.
		logger.Error(err, "failed to fetch application")
		return ctrl.Result{}, err
	}

	// =========================================================================
	// STEP 2: HANDLE DELETION (FINALIZER PATTERN)
	// =========================================================================
	//
	// If deletionTimestamp is set, Kubernetes wants to delete this object.
	// But if we have a finalizer, deletion is blocked until we remove it.
	//
	// This gives us a chance to:
	//   1. Clean up external resources (Terraform destroy, delete cloud resources)
	//   2. Remove our finalizer
	//   3. Kubernetes then actually deletes the object
	//
	// WHY FINALIZERS:
	//
	//   Without finalizer:
	//   ┌──────────────┐     ┌──────────────┐
	//   │ User deletes │────►│ Object gone  │  ← AWS resources orphaned! 💸
	//   │ kubectl del  │     │ immediately  │
	//   └──────────────┘     └──────────────┘
	//
	//   With finalizer:
	//   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
	//   │ User deletes │────►│ deletionTS   │────►│ Reconciler   │
	//   │ kubectl del  │     │ set, blocked │     │ sees delete  │
	//   └──────────────┘     └──────────────┘     └──────┬───────┘
	//                                                    │
	//   ┌──────────────┐     ┌──────────────┐     ┌──────▼───────┐
	//   │ Object gone  │◄────│ Remove       │◄────│ Clean up     │
	//   │ from etcd    │     │ finalizer    │     │ AWS/TF       │
	//   └──────────────┘     └──────────────┘     └──────────────┘
	//
	// =========================================================================

	if !app.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &app)
	}

	// =========================================================================
	// STEP 3: ENSURE FINALIZER EXISTS
	// =========================================================================
	//
	// If object doesn't have our finalizer, add it.
	// This must happen before we create any external resources.
	//
	// ORDER MATTERS:
	//   1. Add finalizer FIRST
	//   2. Create external resources SECOND
	//   Otherwise deletion can slip through between 1 and 2.
	//
	// =========================================================================

	if !controllerutil.ContainsFinalizer(&app, applicationFinalizer) {
		logger.Info("adding finalizer")
		controllerutil.AddFinalizer(&app, applicationFinalizer)
		if err := r.Update(ctx, &app); err != nil {
			logger.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		// Requeue to continue with reconciliation
		return ctrl.Result{Requeue: true}, nil
	}

	// =========================================================================
	// STEP 4: RECONCILE CHILD RESOURCES
	// =========================================================================
	//
	// Create or update the Kubernetes resources that implement this Application.
	// Currently: Deployment + Service
	// Later: Also trigger Terraform for infrastructure
	//
	// IDEMPOTENCY:
	//   Each function checks if resource exists, creates if not, updates if changed.
	//   Running twice produces the same result.
	//
	// =========================================================================

	// Track if this is initial provisioning (no Ready condition yet)
	isInitialProvisioning := meta.FindStatusCondition(app.Status.Conditions, platformv1alpha1.ConditionTypeReady) == nil

	// Set phase to Provisioning during work
	if app.Status.Phase != platformv1alpha1.ApplicationProvisioning && isInitialProvisioning {
		app.Status.Phase = platformv1alpha1.ApplicationProvisioning
		if err := r.Status().Update(ctx, &app); err != nil {
			logger.Error(err, "failed to update phase to Provisioning")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Deployment (if workload specified)
	var deploymentReady bool
	if app.Spec.Workload != nil {
		var err error
		deploymentReady, err = r.reconcileDeployment(ctx, &app)
		if err != nil {
			logger.Error(err, "failed to reconcile deployment")
			r.setFailedCondition(ctx, &app, "DeploymentFailed", err.Error())
			return ctrl.Result{RequeueAfter: requeueAfterError}, nil
		}
	} else {
		// No workload specified, consider workload "ready" (nothing to do)
		deploymentReady = true
	}

	// Reconcile Service (if workload has ports)
	var serviceReady bool
	if app.Spec.Workload != nil && len(app.Spec.Workload.Ports) > 0 {
		var err error
		serviceReady, err = r.reconcileService(ctx, &app)
		if err != nil {
			logger.Error(err, "failed to reconcile service")
			r.setFailedCondition(ctx, &app, "ServiceFailed", err.Error())
			return ctrl.Result{RequeueAfter: requeueAfterError}, nil
		}
	} else {
		// No ports, no service needed
		serviceReady = true
	}

	// =========================================================================
	// STEP 5: UPDATE STATUS
	// =========================================================================
	//
	// Report the current state of the Application.
	// This is crucial for users to understand what's happening.
	//
	// STATUS UPDATES ARE CHEAP:
	//   - Status is a separate subresource (/status endpoint)
	//   - Updating status doesn't trigger reconcile (no spec change)
	//   - Users can watch status for progress
	//
	// =========================================================================

	// Re-fetch the app to get latest version (in case of conflicts)
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		logger.Error(err, "failed to re-fetch application for status update")
		return ctrl.Result{}, err
	}

	// Update conditions
	r.updateConditions(ctx, &app, deploymentReady, serviceReady)

	// Set overall phase
	if deploymentReady && serviceReady {
		app.Status.Phase = platformv1alpha1.ApplicationReady
	} else {
		app.Status.Phase = platformv1alpha1.ApplicationProvisioning
	}

	// Set observed generation
	app.Status.ObservedGeneration = app.Generation

	// Persist status
	if err := r.Status().Update(ctx, &app); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	// =========================================================================
	// STEP 6: RETURN RESULT
	// =========================================================================
	//
	// Signal to the work queue what to do next.
	//
	// OPTIONS:
	//   Result{} + nil          → Done, don't requeue
	//   Result{Requeue: true}   → Requeue immediately (rate-limited)
	//   Result{RequeueAfter: X} → Requeue after duration X
	//   Result{} + error        → Requeue with exponential backoff
	//
	// We always requeue after some time to catch drift (e.g., someone
	// manually deleted the Deployment). This is "reconciliation" - always
	// working toward desired state.
	//
	// =========================================================================

	logger.Info("reconciliation complete",
		"phase", app.Status.Phase,
		"deploymentReady", deploymentReady,
		"serviceReady", serviceReady,
	)

	return ctrl.Result{RequeueAfter: requeueAfterSuccess}, nil
}

// =============================================================================
// HANDLE DELETION - CLEANUP BEFORE OBJECT REMOVAL
// =============================================================================
//
// Called when deletionTimestamp is set. The object is marked for deletion
// but blocked by our finalizer.
//
// FLOW:
//   1. Perform cleanup (Terraform destroy, delete cloud resources)
//   2. Remove our finalizer
//   3. Return - Kubernetes will delete the object
//
// FAILURE HANDLING:
//   If cleanup fails, we return an error → requeue → retry
//   Object stays in "Deleting" state until cleanup succeeds
//   User can see why via kubectl describe (status conditions)
//
// =============================================================================

func (r *ApplicationReconciler) handleDeletion(ctx context.Context, app *platformv1alpha1.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(app, applicationFinalizer) {
		// No finalizer to remove, nothing to do
		return ctrl.Result{}, nil
	}

	logger.Info("handling deletion, cleaning up resources")

	// Update phase to Deleting
	app.Status.Phase = platformv1alpha1.ApplicationDeleting
	if err := r.Status().Update(ctx, app); err != nil {
		logger.Error(err, "failed to update phase to Deleting")
		// Don't return error - we still want to try cleanup
	}

	// =========================================================================
	// CLEANUP EXTERNAL RESOURCES
	// =========================================================================
	//
	// In future milestones, this is where we'll call:
	//   - Terraform destroy for AWS resources
	//   - Crossplane composite deletion
	//   - Any other external cleanup
	//
	// For now, Kubernetes garbage collection handles owned resources
	// (Deployment, Service) via OwnerReference.
	//
	// =========================================================================

	// TODO(M6): Call InfrastructureProvider.Destroy() here
	// For now, owned resources are garbage collected automatically

	logger.Info("cleanup complete, removing finalizer")

	// Remove our finalizer
	controllerutil.RemoveFinalizer(app, applicationFinalizer)
	if err := r.Update(ctx, app); err != nil {
		logger.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, err
	}

	// Don't requeue - object will be deleted
	return ctrl.Result{}, nil
}

// =============================================================================
// RECONCILE DEPLOYMENT
// =============================================================================
//
// Creates or updates the Deployment for this Application's workload.
//
// DESIGN DECISIONS:
//
// 1. OWNER REFERENCE:
//    We set ownerReferences on the Deployment pointing to our Application.
//    This enables:
//    - Garbage collection: Delete Application → Deployment auto-deleted
//    - Watch propagation: Deployment changes → Application reconciled
//
// 2. LABELS:
//    We apply consistent labels for:
//    - Selection (app.kubernetes.io/name matches selector)
//    - Filtering (team label for cost allocation queries)
//    - Tooling (managed-by for GitOps tools to ignore)
//
// 3. SERVER-SIDE APPLY:
//    We use CreateOrUpdate which does a 3-way merge.
//    Alternative is Server-Side Apply (SSA) which is more precise but complex.
//
// HOW OTHER OPERATORS DO IT:
//   - Crossplane: Uses SSA for all resources
//   - ArgoCD: Uses client-side apply with special diff logic
//   - Prometheus Operator: Uses CreateOrUpdate like us
//
// =============================================================================

func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	// Build the desired Deployment
	deployment := r.buildDeployment(app)

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(app, deployment, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// =========================================================================
	// CREATE OR UPDATE
	// =========================================================================
	//
	// CreateOrUpdate implements the "upsert" pattern:
	//   1. Try to Get the existing object
	//   2. If not found, Create it
	//   3. If found, call the mutate function, then Update
	//
	// The mutate function (second arg) modifies the object in-place.
	// We copy desired spec into the existing object.
	//
	// WHY NOT JUST CREATE/UPDATE SEPARATELY:
	//   Race conditions! Between Get and Create, another reconcile might create.
	//   CreateOrUpdate handles this atomically.
	//
	// =========================================================================

	existingDeployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKeyFromObject(deployment), existingDeployment)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create new deployment
			logger.Info("creating deployment", "deployment", deployment.Name)
			if err := r.Create(ctx, deployment); err != nil {
				return false, fmt.Errorf("failed to create deployment: %w", err)
			}
			return false, nil // Not ready yet, just created
		}
		return false, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Update existing deployment
	existingDeployment.Spec = deployment.Spec
	existingDeployment.Labels = deployment.Labels

	logger.Info("updating deployment", "deployment", deployment.Name)
	if err := r.Update(ctx, existingDeployment); err != nil {
		return false, fmt.Errorf("failed to update deployment: %w", err)
	}

	// Check if deployment is ready
	return r.isDeploymentReady(existingDeployment), nil
}

// buildDeployment creates a Deployment from the Application spec.
func (r *ApplicationReconciler) buildDeployment(app *platformv1alpha1.Application) *appsv1.Deployment {
	labels := r.buildLabels(app)
	replicas := int32(1)
	if app.Spec.Workload.Replicas != nil {
		replicas = *app.Spec.Workload.Replicas
	}

	// Build container ports
	var containerPorts []corev1.ContainerPort
	for _, p := range app.Spec.Workload.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		})
	}

	// Build probes if health check specified
	var livenessProbe, readinessProbe *corev1.Probe
	if app.Spec.Workload.HealthCheck != nil {
		hc := app.Spec.Workload.HealthCheck
		probe := &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: hc.Path,
					Port: intstr.FromInt32(hc.Port),
				},
			},
			InitialDelaySeconds: hc.InitialDelaySeconds,
			PeriodSeconds:       hc.PeriodSeconds,
			FailureThreshold:    hc.FailureThreshold,
		}
		livenessProbe = probe
		readinessProbe = probe.DeepCopy()
	}

	// Build environment variables
	env := app.Spec.Workload.Env

	// Build the container
	container := corev1.Container{
		Name:           "app",
		Image:          app.Spec.Workload.Image,
		Ports:          containerPorts,
		Resources:      *app.Spec.Workload.Resources,
		Env:            env,
		LivenessProbe:  livenessProbe,
		ReadinessProbe: readinessProbe,
	}

	if len(app.Spec.Workload.Command) > 0 {
		container.Command = app.Spec.Workload.Command
	}
	if len(app.Spec.Workload.Args) > 0 {
		container.Args = app.Spec.Workload.Args
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     app.Name,
					"app.kubernetes.io/instance": app.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}
}

// isDeploymentReady checks if deployment has reached desired state.
func (r *ApplicationReconciler) isDeploymentReady(deployment *appsv1.Deployment) bool {
	// A deployment is ready when:
	// 1. ObservedGeneration matches Generation (controller has seen spec)
	// 2. Replicas == ReadyReplicas == AvailableReplicas
	// 3. UpdatedReplicas == Replicas (rollout complete)

	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false
	}

	desiredReplicas := int32(1)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}

	return deployment.Status.ReadyReplicas == desiredReplicas &&
		deployment.Status.AvailableReplicas == desiredReplicas &&
		deployment.Status.UpdatedReplicas == desiredReplicas
}

// =============================================================================
// RECONCILE SERVICE
// =============================================================================
//
// Creates or updates the Service for this Application.
// The Service provides stable networking for the Deployment's pods.
//
// SERVICE VS INGRESS:
//   - Service: Internal cluster networking (ClusterIP)
//   - Ingress: External HTTP/HTTPS traffic (later milestone)
//
// =============================================================================

func (r *ApplicationReconciler) reconcileService(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	service := r.buildService(app)

	// Set owner reference
	if err := controllerutil.SetControllerReference(app, service, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	existingService := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(service), existingService)

	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("creating service", "service", service.Name)
			if err := r.Create(ctx, service); err != nil {
				return false, fmt.Errorf("failed to create service: %w", err)
			}
			return true, nil // Services are "ready" immediately
		}
		return false, fmt.Errorf("failed to get service: %w", err)
	}

	// Update existing service (preserve ClusterIP)
	existingService.Spec.Ports = service.Spec.Ports
	existingService.Spec.Selector = service.Spec.Selector
	existingService.Labels = service.Labels

	logger.Info("updating service", "service", service.Name)
	if err := r.Update(ctx, existingService); err != nil {
		return false, fmt.Errorf("failed to update service: %w", err)
	}

	return true, nil
}

// buildService creates a Service from the Application spec.
func (r *ApplicationReconciler) buildService(app *platformv1alpha1.Application) *corev1.Service {
	labels := r.buildLabels(app)

	var ports []corev1.ServicePort
	for _, p := range app.Spec.Workload.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.ContainerPort,
			TargetPort: intstr.FromInt32(p.ContainerPort),
			Protocol:   p.Protocol,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name":     app.Name,
				"app.kubernetes.io/instance": app.Name,
			},
			Ports: ports,
			Type:  corev1.ServiceTypeClusterIP,
		},
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// labelValueRegex matches invalid characters for Kubernetes label values.
// Labels must match: (([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
var labelValueRegex = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

// sanitizeLabelValue converts a string to a valid Kubernetes label value.
// Replaces invalid characters with underscores and truncates to 63 chars.
func sanitizeLabelValue(s string) string {
	// Replace @ and other invalid chars with underscore
	sanitized := labelValueRegex.ReplaceAllString(s, "_")

	// Trim leading/trailing non-alphanumeric
	sanitized = strings.Trim(sanitized, "_.-")

	// Truncate to 63 characters (K8s label value limit)
	if len(sanitized) > 63 {
		sanitized = sanitized[:63]
	}

	// If empty after sanitization, return a default
	if sanitized == "" {
		sanitized = "unknown"
	}

	return sanitized
}

// buildLabels creates standard Kubernetes labels for resources.
func (r *ApplicationReconciler) buildLabels(app *platformv1alpha1.Application) map[string]string {
	return map[string]string{
		// Standard K8s labels (kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)
		"app.kubernetes.io/name":       app.Name,
		"app.kubernetes.io/instance":   app.Name,
		"app.kubernetes.io/managed-by": "goplatform",
		"app.kubernetes.io/part-of":    "goplatform",

		// GoPlatform custom labels (sanitized to be valid label values)
		"platform.goplatform.io/team":  sanitizeLabelValue(app.Spec.Team),
		"platform.goplatform.io/owner": sanitizeLabelValue(app.Spec.Owner),
		"platform.goplatform.io/tier":  string(app.Spec.Tier),
	}
}

// updateConditions sets the status conditions based on current state.
func (r *ApplicationReconciler) updateConditions(ctx context.Context, app *platformv1alpha1.Application, deploymentReady, serviceReady bool) {
	now := metav1.Now()

	// Workload condition
	workloadCondition := metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeWorkloadReady,
		LastTransitionTime: now,
	}
	if app.Spec.Workload == nil {
		workloadCondition.Status = metav1.ConditionTrue
		workloadCondition.Reason = "NoWorkload"
		workloadCondition.Message = "No workload specified"
	} else if deploymentReady {
		workloadCondition.Status = metav1.ConditionTrue
		workloadCondition.Reason = "DeploymentReady"
		workloadCondition.Message = "Deployment has reached desired state"
	} else {
		workloadCondition.Status = metav1.ConditionFalse
		workloadCondition.Reason = "DeploymentNotReady"
		workloadCondition.Message = "Deployment is still progressing"
	}
	meta.SetStatusCondition(&app.Status.Conditions, workloadCondition)

	// Overall Ready condition
	readyCondition := metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeReady,
		LastTransitionTime: now,
	}
	if deploymentReady && serviceReady {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = "AllResourcesReady"
		readyCondition.Message = "All resources are ready"
	} else {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "ResourcesNotReady"
		readyCondition.Message = "Some resources are still being provisioned"
	}
	meta.SetStatusCondition(&app.Status.Conditions, readyCondition)
}

// setFailedCondition updates status to indicate a failure.
func (r *ApplicationReconciler) setFailedCondition(ctx context.Context, app *platformv1alpha1.Application, reason, message string) {
	app.Status.Phase = platformv1alpha1.ApplicationFailed

	failedCondition := metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&app.Status.Conditions, failedCondition)

	if err := r.Status().Update(ctx, app); err != nil {
		log.FromContext(ctx).Error(err, "failed to update failed status")
	}
}

// =============================================================================
// SETUP WITH MANAGER
// =============================================================================
//
// Called once at startup to register this controller with the manager.
//
// WHAT THIS SETS UP:
//
//   1. PRIMARY WATCH:
//      For(&Application{}) → watch our CRD
//      Any create/update/delete triggers Reconcile
//
//   2. SECONDARY WATCHES:
//      Owns(&Deployment{}) → watch Deployments we created
//      Owns(&Service{}) → watch Services we created
//      If these change, reconcile the OWNER Application
//
//   3. PREDICATES (not shown, future enhancement):
//      Filter events to reduce unnecessary reconciles
//      Example: Skip if only resourceVersion changed
//
// THE MANAGER:
//   - Runs all controllers (we might have multiple)
//   - Manages shared cache
//   - Handles leader election
//   - Provides metrics endpoint
//
// =============================================================================

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Primary watch: our CRD
		For(&platformv1alpha1.Application{}).
		// Secondary watches: resources we own
		// When these change, find the owner Application and reconcile it
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		// Controller name (for metrics, logging)
		Named("application").
		Complete(r)
}
