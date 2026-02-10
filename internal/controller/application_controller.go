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
	"reflect"
	"regexp"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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

	// deletionGracePeriod is the maximum time we wait for external resources
	// to be cleaned up before giving up. After this, manual intervention needed.
	// This prevents objects from being stuck in "Deleting" state forever.
	deletionGracePeriod = 30 * time.Minute

	// deletionStartAnnotation tracks when deletion cleanup started.
	// Used to detect stuck deletions that exceed the grace period.
	deletionStartAnnotation = "platform.goplatform.io/deletion-started"
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

	// Recorder emits Kubernetes Events for important operations.
	// =========================================================================
	// WHY EVENTS:
	//   - Events provide operational visibility into controller actions
	//   - kubectl describe shows events (last ~1 hour)
	//   - Monitoring systems can alert on event patterns
	//   - Users understand what the controller is doing
	//
	// WHEN TO EMIT EVENTS:
	//   - Resource creation: "Created Deployment my-app"
	//   - Errors: "Failed to create RDS instance: quota exceeded"
	//   - State transitions: "Scaling from 2 to 4 replicas"
	//   - External actions: "Terraform apply started"
	//
	// EVENT TYPES:
	//   Normal: Routine operations (create, update, scale)
	//   Warning: Errors, degraded state, temporary failures
	//
	// HOW CROSSPLANE/ARGOCD DO IT:
	//   - Crossplane: Events for every resource lifecycle event
	//   - ArgoCD: Events for sync operations, health changes
	//   - Prometheus Operator: Events for config reloads, errors
	// =========================================================================
	Recorder record.EventRecorder
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
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
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

	// Set phase to Provisioning during initial work
	if isInitialProvisioning {
		if err := r.updatePhase(ctx, &app, platformv1alpha1.ApplicationProvisioning); err != nil {
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

	// Reconcile ConfigMap
	if _, err := r.reconcileConfigMap(ctx, &app); err != nil {
		logger.Error(err, "failed to reconcile configmap")
		r.setFailedCondition(ctx, &app, "ConfigMapFailed", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Reconcile Secret
	if _, err := r.reconcileSecret(ctx, &app); err != nil {
		logger.Error(err, "failed to reconcile secret")
		r.setFailedCondition(ctx, &app, "SecretFailed", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Reconcile HPA
	if _, err := r.reconcileHPA(ctx, &app); err != nil {
		logger.Error(err, "failed to reconcile HPA")
		r.setFailedCondition(ctx, &app, "HPAFailed", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// Reconcile PDB
	if _, err := r.reconcilePDB(ctx, &app); err != nil {
		logger.Error(err, "failed to reconcile PDB")
		r.setFailedCondition(ctx, &app, "PDBFailed", err.Error())
		return ctrl.Result{RequeueAfter: requeueAfterError}, nil
	}

	// =========================================================================
	// STEP 5: UPDATE STATUS WITH RETRY
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
	// WHY RETRY:
	// =========================================================================
	//   Status updates can fail due to conflicts (another reconcile updated
	//   the object). Instead of failing the entire reconcile, we retry with
	//   exponential backoff.
	//
	//   CONFLICT SCENARIO:
	//   1. Reconcile A reads app (resourceVersion: 100)
	//   2. Reconcile B reads app (resourceVersion: 100)
	//   3. Reconcile A updates status (resourceVersion: 101)
	//   4. Reconcile B tries to update status → CONFLICT! (rv 100 < 101)
	//   5. With retry, B re-fetches (rv 101) and updates → Success
	//
	//   HOW CROSSPLANE/ARGOCD HANDLE IT:
	//     - Crossplane: Uses RetryOnConflict for all status updates
	//     - ArgoCD: Similar pattern with retry wrapper
	//     - Prometheus Operator: Implements retry loop
	// =========================================================================

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch the app to get latest version
		if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
			return err
		}

		originalStatus := app.Status

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

		// Persist status only when something changed
		if reflect.DeepEqual(originalStatus, app.Status) {
			return nil
		}

		// Persist status
		return r.Status().Update(ctx, &app)
	})

	if retryErr != nil {
		logger.Error(retryErr, "failed to update status after retries")
		return ctrl.Result{}, retryErr
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
// TIMEOUT HANDLING:
// =============================================================================
//   Objects stuck in deleting state (e.g., Terraform keeps failing) can
//   cause operational issues. We track when deletion started and emit
//   warning events if it exceeds the grace period.
//
//   This doesn't force-delete (that would orphan resources), but provides
//   visibility for operators to investigate.
// =============================================================================
//
// =============================================================================

func (r *ApplicationReconciler) handleDeletion(ctx context.Context, app *platformv1alpha1.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(app, applicationFinalizer) {
		// No finalizer to remove, nothing to do
		return ctrl.Result{}, nil
	}

	logger.Info("handling deletion, cleaning up resources")

	// =========================================================================
	// TRACK DELETION START TIME
	// =========================================================================
	//
	// We add an annotation to track when cleanup started.
	// If cleanup takes longer than the grace period, we emit warnings.
	//
	// =========================================================================

	if app.Annotations == nil {
		app.Annotations = make(map[string]string)
	}

	if _, exists := app.Annotations[deletionStartAnnotation]; !exists {
		app.Annotations[deletionStartAnnotation] = time.Now().Format(time.RFC3339)
		if err := r.Update(ctx, app); err != nil {
			logger.Error(err, "failed to set deletion start annotation")
			return ctrl.Result{}, err
		}
		// Requeue to continue after annotation is set
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if deletion is taking too long
	if startTimeStr, exists := app.Annotations[deletionStartAnnotation]; exists {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err == nil && time.Since(startTime) > deletionGracePeriod {
			logger.Info("deletion exceeds grace period",
				"gracePeriod", deletionGracePeriod,
				"elapsed", time.Since(startTime))
			r.Recorder.Event(app, corev1.EventTypeWarning, "DeletionStuck",
				fmt.Sprintf("Deletion cleanup exceeds %v grace period. Manual intervention may be required.",
					deletionGracePeriod))
		}
	}

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
// 3. CreateOrUpdate PATTERN:
//    =========================================================================
//    WHY CreateOrUpdate:
//      - Atomic: No race condition between check and create/update
//      - Idempotent: Safe to call multiple times with same result
//      - Clean: One function handles both create and update paths
//
//    HOW IT WORKS:
//      1. Fetches existing object (or creates empty template)
//      2. Calls our mutate function to set desired spec
//      3. Either Creates (if new) or Updates (if existing)
//      4. Returns operation result (Created, Updated, Unchanged)
//
//    ALTERNATIVES:
//    ┌─────────────────────────────────────────────────────────────────────┐
//    │ Approach          │ Pros                 │ Cons                     │
//    ├───────────────────┼──────────────────────┼──────────────────────────┤
//    │ Get/Create/Update │ Fine control         │ Race conditions possible │
//    │ (what we had)     │                      │ between Get and Create   │
//    ├───────────────────┼──────────────────────┼──────────────────────────┤
//    │ Server-Side Apply │ Best for conflicts   │ More complex, requires   │
//    │ (SSA)             │ Multi-owner support  │ field manager setup      │
//    ├───────────────────┼──────────────────────┼──────────────────────────┤
//    │ ✅ CreateOrUpdate │ Atomic, simple,      │ Full replacement, not    │
//    │                   │ idempotent           │ merge                    │
//    └─────────────────────────────────────────────────────────────────────┘
//
//    HOW CROSSPLANE/ARGOCD DO IT:
//      - Crossplane: Uses SSA for fine-grained field ownership
//      - ArgoCD: Uses client-side apply with special diff logic
//      - Prometheus Operator: Uses CreateOrUpdate like us
//    =========================================================================
//
// =============================================================================

func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	// =========================================================================
	// BUILD DESIRED STATE
	// =========================================================================
	desired := r.buildDeployment(app)

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// =========================================================================
	// CREATE OR UPDATE ATOMICALLY
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
	// IMPORTANT: The object passed to CreateOrUpdate must have ObjectMeta
	// (Name, Namespace) set. The mutate function receives the existing
	// object (or empty template) and should update it to desired state.
	//
	// =========================================================================

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// This function mutates 'deployment' to match desired state
		// It's called after Get (if exists) or with empty object (if new)

		// Copy spec from desired
		deployment.Spec = desired.Spec
		deployment.Labels = desired.Labels

		// Set owner reference (needed inside mutate for create case)
		return controllerutil.SetControllerReference(app, deployment, r.Scheme)
	})

	if err != nil {
		return false, fmt.Errorf("failed to create/update deployment: %w", err)
	}

	// Emit event based on operation result
	switch opResult {
	case controllerutil.OperationResultCreated:
		logger.Info("created deployment", "deployment", deployment.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "DeploymentCreated",
			fmt.Sprintf("Created Deployment %s", deployment.Name))
	case controllerutil.OperationResultUpdated:
		logger.Info("updated deployment", "deployment", deployment.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "DeploymentUpdated",
			fmt.Sprintf("Updated Deployment %s", deployment.Name))
	case controllerutil.OperationResultNone:
		// No changes needed
	}

	// Check if deployment is ready
	return r.isDeploymentReady(deployment), nil
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

	// Build resource requirements (with nil-safe handling)
	// =========================================================================
	// NIL SAFETY:
	//   Resources might be nil if user didn't specify limits/requests.
	//   Instead of panicking, we use an empty ResourceRequirements.
	//   Kubernetes will use defaults from LimitRange or none.
	//
	// BEST PRACTICE:
	//   Always check for nil before dereferencing pointers.
	//   This prevents runtime panics and makes tests more robust.
	// =========================================================================
	var resources corev1.ResourceRequirements
	if app.Spec.Workload.Resources != nil {
		resources = *app.Spec.Workload.Resources
	}

	// Build the container
	container := corev1.Container{
		Name:           "app",
		Image:          app.Spec.Workload.Image,
		Ports:          containerPorts,
		Resources:      resources,
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

	desired := r.buildService(app)

	// Set owner reference
	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Preserve ClusterIP on update (immutable field)
		clusterIP := service.Spec.ClusterIP

		service.Spec = desired.Spec
		service.Labels = desired.Labels

		// Restore ClusterIP if it was set (empty string means new service)
		if clusterIP != "" {
			service.Spec.ClusterIP = clusterIP
		}

		return controllerutil.SetControllerReference(app, service, r.Scheme)
	})

	if err != nil {
		return false, fmt.Errorf("failed to create/update service: %w", err)
	}

	switch opResult {
	case controllerutil.OperationResultCreated:
		logger.Info("created service", "service", service.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "ServiceCreated",
			fmt.Sprintf("Created Service %s", service.Name))
	case controllerutil.OperationResultUpdated:
		logger.Info("updated service", "service", service.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "ServiceUpdated",
			fmt.Sprintf("Updated Service %s", service.Name))
	}

	return true, nil // Services are "ready" immediately
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
// RECONCILE CONFIGMAP
// =============================================================================
//
// Creates or updates the ConfigMap for this Application.
// Used for non-sensitive configuration.
//
// =============================================================================

func (r *ApplicationReconciler) reconcileConfigMap(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	desired := r.buildConfigMap(app)

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = desired.Data
		cm.Labels = desired.Labels
		return controllerutil.SetControllerReference(app, cm, r.Scheme)
	})

	if err != nil {
		return false, fmt.Errorf("failed to create/update configmap: %w", err)
	}

	switch opResult {
	case controllerutil.OperationResultCreated:
		logger.Info("created configmap", "configmap", cm.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "ConfigMapCreated",
			fmt.Sprintf("Created ConfigMap %s", cm.Name))
	case controllerutil.OperationResultUpdated:
		logger.Info("updated configmap", "configmap", cm.Name)
	}

	return true, nil
}

func (r *ApplicationReconciler) buildConfigMap(app *platformv1alpha1.Application) *corev1.ConfigMap {
	labels := r.buildLabels(app)

	data := map[string]string{
		"APP_NAME": app.Name,
		"APP_TEAM": app.Spec.Team,
		"APP_TIER": string(app.Spec.Tier),
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

// =============================================================================
// RECONCILE SECRET
// =============================================================================
//
// Creates the Secret for this Application.
// Acts as a placeholder for sensitive data and infrastructure credentials.
//
// NOTE: We only create secrets, we don't update them after initial creation.
// This prevents overwriting secrets that were populated by external systems
// (like Terraform outputs, external-secrets operator, etc.)
//
// =============================================================================

func (r *ApplicationReconciler) reconcileSecret(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	desired := r.buildSecret(app)

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		// Only set data on creation, not update
		// This prevents overwriting secrets populated by external systems
		if secret.CreationTimestamp.IsZero() {
			secret.Data = desired.Data
			secret.Type = desired.Type
		}
		secret.Labels = desired.Labels
		return controllerutil.SetControllerReference(app, secret, r.Scheme)
	})

	if err != nil {
		return false, fmt.Errorf("failed to create/update secret: %w", err)
	}

	if opResult == controllerutil.OperationResultCreated {
		logger.Info("created secret", "secret", secret.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "SecretCreated",
			fmt.Sprintf("Created Secret %s", secret.Name))
	}

	return true, nil
}

func (r *ApplicationReconciler) buildSecret(app *platformv1alpha1.Application) *corev1.Secret {
	labels := r.buildLabels(app)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			// Placeholder - normally would be populated by infrastructure providers
		},
	}
}

// =============================================================================
// RECONCILE HPA
// =============================================================================
//
// Creates or updates the HorizontalPodAutoscaler.
// Deletes HPA if scaling spec is removed (consistent cleanup).
//
// =============================================================================

func (r *ApplicationReconciler) reconcileHPA(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	// =========================================================================
	// CLEANUP LOGIC: Delete HPA if scaling spec is removed
	// =========================================================================
	//
	// WHY WE NEED THIS:
	//   Owner references handle deletion when the Application is deleted,
	//   but if user just removes spec.Scaling, the HPA would be orphaned.
	//   We explicitly delete it to maintain consistency.
	//
	// =========================================================================

	if app.Spec.Scaling == nil {
		existingHPA := &autoscalingv2.HorizontalPodAutoscaler{}
		err := r.Get(ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, existingHPA)

		if err == nil {
			// Found HPA but scaling spec is nil -> Delete it
			logger.Info("removing HPA as scaling spec is nil", "hpa", app.Name)
			if err := r.Delete(ctx, existingHPA); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("failed to delete HPA: %w", err)
			}
			r.Recorder.Event(app, corev1.EventTypeNormal, "HPADeleted",
				fmt.Sprintf("Deleted HPA %s (scaling spec removed)", app.Name))
		}
		return true, nil
	}

	desired := r.buildHPA(app)

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
		hpa.Spec = desired.Spec
		hpa.Labels = desired.Labels
		return controllerutil.SetControllerReference(app, hpa, r.Scheme)
	})

	if err != nil {
		return false, fmt.Errorf("failed to create/update HPA: %w", err)
	}

	switch opResult {
	case controllerutil.OperationResultCreated:
		logger.Info("created HPA", "hpa", hpa.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "HPACreated",
			fmt.Sprintf("Created HPA %s (min: %d, max: %d)",
				hpa.Name, *app.Spec.Scaling.MinReplicas, app.Spec.Scaling.MaxReplicas))
	case controllerutil.OperationResultUpdated:
		logger.Info("updated HPA", "hpa", hpa.Name)
	}

	return true, nil
}

func (r *ApplicationReconciler) buildHPA(app *platformv1alpha1.Application) *autoscalingv2.HorizontalPodAutoscaler {
	labels := r.buildLabels(app)
	minReplicas := int32(1)
	if app.Spec.Scaling.MinReplicas != nil {
		minReplicas = *app.Spec.Scaling.MinReplicas
	}

	var metrics []autoscalingv2.MetricSpec
	for _, m := range app.Spec.Scaling.Metrics {
		if m.Type == "cpu" {
			metrics = append(metrics, autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &m.Target,
					},
				},
			})
		} else if m.Type == "memory" {
			metrics = append(metrics, autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceMemory,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &m.Target,
					},
				},
			})
		}
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       app.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: app.Spec.Scaling.MaxReplicas,
			Metrics:     metrics,
		},
	}
}

// =============================================================================
// RECONCILE PDB
// =============================================================================
//
// Creates or updates the PodDisruptionBudget.
// Ensures availability during voluntary disruptions (upgrades, draining).
// Deletes PDB if tier changes to development (consistent cleanup).
//
// PDB SPEC MUTABILITY (K8s 1.27+):
//   PDB spec is now mutable! We can update minAvailable/maxUnavailable.
//   Previously (K8s < 1.27), PDB spec was immutable after creation.
//
// =============================================================================

func (r *ApplicationReconciler) reconcilePDB(ctx context.Context, app *platformv1alpha1.Application) (bool, error) {
	logger := log.FromContext(ctx)

	// =========================================================================
	// CLEANUP LOGIC: Delete PDB if tier is development or no workload
	// =========================================================================
	//
	// WHY:
	//   - Development tier doesn't need availability guarantees
	//   - No workload means no pods to protect
	//   - Consistent cleanup like HPA
	//
	// =========================================================================

	if app.Spec.Tier == platformv1alpha1.TierDevelopment || app.Spec.Workload == nil {
		existingPDB := &policyv1.PodDisruptionBudget{}
		err := r.Get(ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, existingPDB)

		if err == nil {
			// Found PDB but tier is development or no workload -> Delete it
			logger.Info("removing PDB (tier=development or no workload)", "pdb", app.Name)
			if err := r.Delete(ctx, existingPDB); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("failed to delete PDB: %w", err)
			}
			r.Recorder.Event(app, corev1.EventTypeNormal, "PDBDeleted",
				fmt.Sprintf("Deleted PDB %s (tier changed to development)", app.Name))
		}
		return true, nil
	}

	desired := r.buildPDB(app)

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference: %w", err)
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, pdb, func() error {
		pdb.Spec = desired.Spec
		pdb.Labels = desired.Labels
		return controllerutil.SetControllerReference(app, pdb, r.Scheme)
	})

	if err != nil {
		return false, fmt.Errorf("failed to create/update PDB: %w", err)
	}

	switch opResult {
	case controllerutil.OperationResultCreated:
		logger.Info("created PDB", "pdb", pdb.Name)
		r.Recorder.Event(app, corev1.EventTypeNormal, "PDBCreated",
			fmt.Sprintf("Created PDB %s for %s tier", pdb.Name, app.Spec.Tier))
	case controllerutil.OperationResultUpdated:
		logger.Info("updated PDB", "pdb", pdb.Name)
	}

	return true, nil
}

func (r *ApplicationReconciler) buildPDB(app *platformv1alpha1.Application) *policyv1.PodDisruptionBudget {
	labels := r.buildLabels(app)

	// Default to minAvailable 1 for high availability
	minAvailable := intstr.FromInt(1)

	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     app.Name,
					"app.kubernetes.io/instance": app.Name,
				},
			},
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
	observedGeneration := app.Generation

	// =========================================================================
	// WORKLOAD CONDITION
	// =========================================================================
	//
	// If no workload is specified, we treat workload as ready (infra-only app).
	// Otherwise, readiness reflects Deployment status.
	// =========================================================================

	workloadStatus := metav1.ConditionFalse
	workloadReason := "DeploymentNotReady"
	workloadMessage := "Deployment is still progressing"

	if app.Spec.Workload == nil {
		workloadStatus = metav1.ConditionTrue
		workloadReason = "NoWorkload"
		workloadMessage = "No workload specified"
	} else if deploymentReady {
		workloadStatus = metav1.ConditionTrue
		workloadReason = "DeploymentReady"
		workloadMessage = "Deployment has reached desired state"
	}

	workloadCondition := platformv1alpha1.NewCondition(
		platformv1alpha1.ConditionTypeWorkloadReady,
		workloadStatus,
		workloadReason,
		workloadMessage,
		observedGeneration,
	)
	r.applyCondition(ctx, app, workloadCondition)

	// =========================================================================
	// INFRASTRUCTURE CONDITIONS
	// =========================================================================
	//
	// At this milestone we haven't implemented provisioning yet.
	// We still report conditions so users can see *what is expected*.
	//
	// - If a component is NOT requested, its condition is True (NotRequested)
	// - If a component IS requested, its condition is False (ProvisioningPending)
	//
	// This mirrors how Crossplane reports Ready=False while resources are
	// being created, even if the provider is not yet wired in.
	// =========================================================================

	databaseReady := r.setInfrastructureCondition(
		ctx,
		app,
		platformv1alpha1.ConditionTypeDatabaseReady,
		app.Spec.Database != nil,
		"Database",
	)
	cacheReady := r.setInfrastructureCondition(
		ctx,
		app,
		platformv1alpha1.ConditionTypeCacheReady,
		app.Spec.Cache != nil,
		"Cache",
	)
	queueReady := r.setInfrastructureCondition(
		ctx,
		app,
		platformv1alpha1.ConditionTypeQueueReady,
		app.Spec.Queue != nil,
		"Queue",
	)
	storageReady := r.setInfrastructureCondition(
		ctx,
		app,
		platformv1alpha1.ConditionTypeStorageReady,
		app.Spec.Storage != nil,
		"Storage",
	)

	// Ensure Infrastructure status object exists when any infra is requested
	r.ensureInfrastructureStatus(app)

	// =========================================================================
	// OVERALL READY CONDITION
	// =========================================================================

	infraReady := databaseReady && cacheReady && queueReady && storageReady
	overallReady := workloadStatus == metav1.ConditionTrue && serviceReady && infraReady

	readyReason := "ResourcesNotReady"
	readyMessage := "Some resources are still being provisioned"
	if overallReady {
		readyReason = "AllResourcesReady"
		readyMessage = "All resources are ready"
	} else if !serviceReady {
		readyReason = "ServiceNotReady"
		readyMessage = "Service is not ready"
	} else if workloadStatus != metav1.ConditionTrue {
		readyReason = "WorkloadNotReady"
		readyMessage = "Workload is not ready"
	} else if !infraReady {
		readyReason = "InfrastructureNotReady"
		readyMessage = "Infrastructure provisioning is pending"
	}

	readyStatus := metav1.ConditionFalse
	if overallReady {
		readyStatus = metav1.ConditionTrue
	}

	readyCondition := platformv1alpha1.NewCondition(
		platformv1alpha1.ConditionTypeReady,
		readyStatus,
		readyReason,
		readyMessage,
		observedGeneration,
	)
	r.applyCondition(ctx, app, readyCondition)
}

// setFailedCondition updates status to indicate a failure.
func (r *ApplicationReconciler) setFailedCondition(ctx context.Context, app *platformv1alpha1.Application, reason, message string) {
	if app == nil {
		return
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest platformv1alpha1.Application
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), &latest); err != nil {
			return err
		}

		originalStatus := latest.Status

		latest.Status.Phase = platformv1alpha1.ApplicationFailed
		latest.Status.ObservedGeneration = latest.Generation

		failedCondition := platformv1alpha1.NewCondition(
			platformv1alpha1.ConditionTypeReady,
			metav1.ConditionFalse,
			reason,
			message,
			latest.Generation,
		)
		r.applyCondition(ctx, &latest, failedCondition)

		if reflect.DeepEqual(originalStatus, latest.Status) {
			return nil
		}

		return r.Status().Update(ctx, &latest)
	})

	if retryErr != nil {
		log.FromContext(ctx).Error(retryErr, "failed to update failed status")
	}
}

// updatePhase updates the status phase with retry-on-conflict handling.
func (r *ApplicationReconciler) updatePhase(ctx context.Context, app *platformv1alpha1.Application, phase platformv1alpha1.ApplicationPhase) error {
	if app == nil {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest platformv1alpha1.Application
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), &latest); err != nil {
			return err
		}

		if latest.Status.Phase == phase {
			return nil
		}

		originalStatus := latest.Status
		latest.Status.Phase = phase
		latest.Status.ObservedGeneration = latest.Generation

		if reflect.DeepEqual(originalStatus, latest.Status) {
			return nil
		}

		return r.Status().Update(ctx, &latest)
	})
}

// applyCondition sets a condition and emits an Event when the status changes.
func (r *ApplicationReconciler) applyCondition(ctx context.Context, app *platformv1alpha1.Application, condition metav1.Condition) {
	if app == nil {
		return
	}

	previous := platformv1alpha1.GetCondition(app.Status.Conditions, condition.Type)
	previousStatus := metav1.ConditionUnknown
	if previous != nil {
		previousStatus = previous.Status
	}

	changed := platformv1alpha1.SetCondition(&app.Status.Conditions, condition)
	if !changed || previousStatus == condition.Status {
		return
	}

	if r.Recorder == nil {
		return
	}

	eventType := corev1.EventTypeNormal
	if condition.Status == metav1.ConditionFalse {
		eventType = corev1.EventTypeWarning
	}

	// Use the condition reason as the event reason for consistency.
	r.Recorder.Event(app, eventType, condition.Reason, fmt.Sprintf("%s: %s", condition.Type, condition.Message))
}

// setInfrastructureCondition sets a standard condition for infra components.
// Returns true if the component is considered ready.
func (r *ApplicationReconciler) setInfrastructureCondition(
	ctx context.Context,
	app *platformv1alpha1.Application,
	conditionType string,
	requested bool,
	componentName string,
) bool {
	status := metav1.ConditionFalse
	reason := fmt.Sprintf("%sProvisioningPending", componentName)
	message := fmt.Sprintf("%s provisioning is pending", componentName)

	if !requested {
		status = metav1.ConditionTrue
		reason = fmt.Sprintf("%sNotRequested", componentName)
		message = fmt.Sprintf("%s not requested", componentName)
	}

	condition := platformv1alpha1.NewCondition(
		conditionType,
		status,
		reason,
		message,
		app.Generation,
	)

	r.applyCondition(ctx, app, condition)
	return status == metav1.ConditionTrue
}

// ensureInfrastructureStatus initializes Infrastructure status when needed.
func (r *ApplicationReconciler) ensureInfrastructureStatus(app *platformv1alpha1.Application) {
	if app == nil || app.Status.Infrastructure != nil {
		return
	}

	if app.Spec.Database != nil || app.Spec.Cache != nil || app.Spec.Queue != nil || app.Spec.Storage != nil {
		app.Status.Infrastructure = &platformv1alpha1.InfrastructureStatus{}
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
//   3. PREDICATES:
//      =========================================================================
//      Filter events to reduce unnecessary reconciles.
//
//      WHY PREDICATES MATTER:
//        Without predicates, we reconcile on ANY update, including:
//        - Status-only changes (we caused these!)
//        - Metadata-only changes (labels, annotations)
//        - resourceVersion changes (every API write)
//
//        With GenerationChangedPredicate:
//        - Only reconcile when spec changes (Generation increments)
//        - Status updates don't trigger reconcile (ObservedGeneration != Generation)
//        - Greatly reduces unnecessary work
//
//      ALTERNATIVES CONSIDERED:
//      ┌────────────────────────────────────────────────────────────────────────┐
//      │ Predicate                 │ Filters Out                                │
//      ├───────────────────────────┼────────────────────────────────────────────┤
//      │ GenerationChangedPred.    │ Status-only updates                        │
//      │ AnnotationChangedPred.    │ Label/annotation-only updates              │
//      │ ResourceVersionChangedP.  │ Almost nothing (too broad)                 │
//      │ Custom Predicate          │ Whatever logic you define                  │
//      └────────────────────────────────────────────────────────────────────────┘
//
//      HOW CROSSPLANE/ARGOCD DO IT:
//        - Crossplane: Uses GenerationChanged + custom predicates
//        - ArgoCD: Custom predicates for specific fields
//        - Prometheus Operator: GenerationChanged + annotation predicates
//      =========================================================================
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
		// Primary watch: our CRD with generation predicate
		// Only reconcile when spec changes, not on status updates
		For(&platformv1alpha1.Application{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Secondary watches: resources we own
		// When these change, find the owner Application and reconcile it
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		// Controller name (for metrics, logging)
		Named("application").
		Complete(r)
}
