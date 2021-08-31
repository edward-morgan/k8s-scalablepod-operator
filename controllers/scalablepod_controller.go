/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	scalablev1 "github.com/edwmorgan/k8s-operator-example/api/v1"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
)

// ScalablePodReconciler reconciles a ScalablePod object
type ScalablePodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=scalable.scalablepod.tutorial.io,resources=scalablepods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scalable.scalablepod.tutorial.io,resources=scalablepods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scalable.scalablepod.tutorial.io,resources=scalablepods/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

func (r *ScalablePodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("scalablepod", req.NamespacedName)

	var scalablePod scalablev1.ScalablePod
	if err := r.Get(ctx, req.NamespacedName, &scalablePod); err != nil {
		r.Log.Info("Unable to find ScalablePod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch scalablePod.Status.Status {
	case scalablev1.SPActive:
		shutdownDuration, _ := time.ParseDuration(fmt.Sprintf("%ds", scalablePod.Spec.MaxReadyTimeSec))
		shutdownTime := scalablePod.Status.StartedAt.Add(shutdownDuration)
		// If the SP needs a Pod to be bound to it
		if scalablePod.Status.BoundPod == nil {
			podName := uuid.New().String()
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           fmt.Sprintf("%s:%s", scalablePod.Spec.PodImageName, scalablePod.Spec.PodImageTag),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sleep",
								"3600",
							},
						},
					},
				},
			}
			r.Log.Info("Creating pod `name`", pod.Name)
			r.Client.Create(ctx, pod)
			scalablePod.Status.BoundPod = &pod.ObjectMeta
			scalablePod.Status.StartedAt = metav1.Now()
		} else if shutdownTime.Before(metav1.Now().Time) { // We need to spin down this ScalablePod
			var pod corev1.Pod
			r.Get(ctx, types.NamespacedName{Namespace: scalablePod.Status.BoundPod.Namespace, Name: scalablePod.Status.BoundPod.Name}, &pod)
			r.Log.Info("Shutting down pod w/name NAME", "NAME", pod.Name)
			r.Client.Delete(ctx, pod.DeepCopy())
			scalablePod.Status.BoundPod = nil
		}
	case scalablev1.SPInactive:
		// If there's still a bound pod, remove it
		if scalablePod.Status.BoundPod != nil {
			var pod corev1.Pod
			r.Get(ctx, types.NamespacedName{Namespace: scalablePod.Status.BoundPod.Namespace, Name: scalablePod.Status.BoundPod.Name}, &pod)
			r.Log.Info("Removing bound pod w/name NAME from inactive ScalablePod", "NAME", pod.Name)
			r.Client.Delete(ctx, pod.DeepCopy())
			scalablePod.Status.BoundPod = nil
		}
	}

	// TODO: Should this be configurable?
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalablePodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scalablev1.ScalablePod{}).
		Owns(&corev1.Pod{}). // TODO: Does this bind our controller to all pods?
		Complete(r)
}
