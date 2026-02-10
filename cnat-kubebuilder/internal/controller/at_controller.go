/*
Copyright 2026 Programming Kubernetes authors.

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
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cnatv1alpha1 "Kubernetes_Programming/api/v1alpha1"
)

// AtReconciler reconciles a At object
type AtReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cnat.programming-kubernetes.info,resources=ats,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cnat.programming-kubernetes.info,resources=ats/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cnat.programming-kubernetes.info,resources=ats/finalizers,verbs=update

// Reconcile is the CORE of the controller - it's called automatically by Kubernetes whenever:
// 1. An At resource is created, updated, or deleted
// 2. A Pod owned by an At resource changes (due to SetupWithManager's For/Owns)
// 3. The RequeueAfter duration expires (if we return one)
// 4. Manual requeue is triggered
//
// RECONCILE PRINCIPLE:
// - Input: Request (namespace + name of the resource that changed)
// - Output: (Result, error) - tells k8s WHEN to call Reconcile again
// - Goal: Make actual state match desired state (spec -> status)
//
// RETURN VALUES EXPLAINED:
//
//  1. reconcile.Result{}, nil
//     → Success! Don't requeue. Will reconcile again only if resource changes.
//
//  2. reconcile.Result{}, err
//     → Error! Requeue with exponential backoff (1s, 2s, 4s, 8s... up to 5min)
//
//  3. reconcile.Result{Requeue: true}, nil
//     → Success, but requeue immediately (rarely used)
//
//  4. reconcile.Result{RequeueAfter: duration}, nil
//     → Success! Call Reconcile again after specified duration
//     → Used for time-based operations (like our scheduled command)
//
//  5. reconcile.Result{RequeueAfter: duration}, err
//     → Error wins! Ignores RequeueAfter, uses error backoff
func (r *AtReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx).WithValues("namespace", req.Namespace, "at", req.Name)
	reqLogger.Info("=== Reconciling At")
	// Fetch the At instance
	instance := &cnatv1alpha1.At{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after
			// reconcile request—return and don't requeue:
			return reconcile.Result{}, nil
		}
		// Error reading the object—requeue the request:
		return reconcile.Result{}, err
	}
	// If no phase set, default to pending (the initial phase):
	if instance.Status.Phase == "" {
		instance.Status.Phase = cnatv1alpha1.PhasePending
	}
	// STATE MACHINE: PENDING -> RUNNING -> DONE
	// Each reconcile call processes current phase and potentially transitions to next
	switch instance.Status.Phase {
	case cnatv1alpha1.PhasePending:
		reqLogger.Info("Phase: PENDING")
		// PENDING: Resource created but scheduled time hasn't arrived yet
		reqLogger.Info("Checking schedule", "Target", instance.Spec.Schedule)

		// Calculate how long until the scheduled time
		d, err := timeUntilSchedule(instance.Spec.Schedule)
		if err != nil {
			reqLogger.Error(err, "Schedule parsing failure")
			// RETURN: reconcile.Result{}, err
			// → Requeue with exponential backoff until user fixes the schedule
			return reconcile.Result{}, err
		}
		reqLogger.Info("Schedule parsing done", "diff", fmt.Sprintf("%v", d))

		if d > 0 {
			// Schedule is in the future (e.g., 5 minutes from now)
			// RETURN: reconcile.Result{RequeueAfter: d}, nil
			// → Sleep for exactly 'd' duration, then Reconcile will run again
			// → This is EFFICIENT - we don't poll, Kubernetes wakes us up at the right time
			reqLogger.Info("Scheduling reconcile", "after", d)
			return reconcile.Result{RequeueAfter: d}, nil
		}

		// Time has arrived! Transition to RUNNING phase
		reqLogger.Info("It's time!", "Ready to execute", instance.Spec.Command)
		instance.Status.Phase = cnatv1alpha1.PhaseRunning
		// Note: We DON'T return here - we fall through to update status at the end
	case cnatv1alpha1.PhaseRunning:
		reqLogger.Info("Phase: RUNNING")
		// RUNNING: We need to create a Pod to execute the command

		pod := newPodForCR(instance)
		// Set At instance as the owner - when At is deleted, Pod is auto-deleted (Garbage Collection)
		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			// RETURN: reconcile.Result{}, err
			// → Requeue with backoff due to error
			return reconcile.Result{}, err
		}

		// Check if the pod already exists
		found := &corev1.Pod{}
		nsName := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
		err = r.Get(context.TODO(), nsName, found)

		if err != nil && errors.IsNotFound(err) {
			// Pod doesn't exist yet - create it!
			err = r.Create(context.TODO(), pod)
			if err != nil {
				// RETURN: reconcile.Result{}, err
				// → Creation failed, requeue with backoff
				return reconcile.Result{}, err
			}
			reqLogger.Info("Pod launched", "name", pod.Name)
			// RETURN: reconcile.Result{}, nil (falls through at end)
			// → Pod created successfully
			// → Reconcile will run again when Pod status changes (due to SetupWithManager)
		} else if err != nil {
			// RETURN: reconcile.Result{}, err
			// → Error getting pod, requeue with backoff
			return reconcile.Result{}, err
		} else if found.Status.Phase == corev1.PodFailed ||
			found.Status.Phase == corev1.PodSucceeded {
			// Pod finished executing! Transition to DONE
			reqLogger.Info("Container terminated", "reason",
				found.Status.Reason, "message", found.Status.Message)
			instance.Status.Phase = cnatv1alpha1.PhaseDone
			// Note: We DON'T return here - we fall through to update status at the end
		} else {
			// Pod is still running (Pending/Running phase)
			// RETURN: reconcile.Result{}, nil
			// → Don't requeue manually
			// → Kubernetes will automatically call Reconcile when Pod status changes
			//   (because we set owner reference and watch Pods in SetupWithManager)
			reqLogger.Info("Pod still running", "phase", found.Status.Phase)
			return reconcile.Result{}, nil
		}
	case cnatv1alpha1.PhaseDone:
		reqLogger.Info("Phase: DONE")
		// DONE: Command executed, nothing more to do
		// RETURN: reconcile.Result{}, nil
		// → Success, don't requeue
		// → Will only reconcile if someone manually edits the resource
		return reconcile.Result{}, nil
	default:
		reqLogger.Info("NOP")
		return reconcile.Result{}, nil
	}

	// Update the At instance status in Kubernetes
	// This is called when we transition phases (PENDING→RUNNING or RUNNING→DONE)
	err = r.Status().Update(context.TODO(), instance)
	if err != nil {
		// RETURN: reconcile.Result{}, err
		// → Status update failed, requeue with backoff
		return reconcile.Result{}, err
	}

	// RETURN: reconcile.Result{}, nil
	// → Status updated successfully
	// → Don't requeue - wait for next event (Pod change or manual edit)
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AtReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cnatv1alpha1.At{}).
		Named("at").
		Complete(r)
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *cnatv1alpha1.At) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: strings.Split(cr.Spec.Command, " "),
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
}

// timeUntilSchedule parses the schedule string and returns the time until the schedule.
// When it is overdue, the duration is negative.
func timeUntilSchedule(schedule string) (time.Duration, error) {
	now := time.Now().UTC()
	layout := "2006-01-02T15:04:05Z"
	s, err := time.Parse(layout, schedule)
	if err != nil {
		return time.Duration(0), err
	}
	return s.Sub(now), nil
}
