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
	"log"
	"time"

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
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=scalable.scalablepod.tutorial.io,resources=scalablepods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scalable.scalablepod.tutorial.io,resources=scalablepods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scalable.scalablepod.tutorial.io,resources=scalablepods/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get

func (r *ScalablePodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var scalablePod scalablev1.ScalablePod
	if err := r.Get(ctx, req.NamespacedName, &scalablePod); err != nil {
		log.Println("Unable to find ScalablePod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Printf("Reconciling ScalablePod `%s`\n", scalablePod.Name)
	switch {
	// If the SP was just added, it won't have a Status
	case scalablePod.Status.Status == nil:
		log.Printf("ScalablePod %s/%s is new. Initializing...\n", scalablePod.Namespace, scalablePod.Name)
		scalablePod.Status.Status = new(scalablev1.SPStatus)
		*scalablePod.Status.Status = scalablev1.SPInactive
		if err := r.Status().Update(ctx, &scalablePod); err != nil {
			log.Println("Unable to update ScalablePod status")
			return ctrl.Result{Requeue: true}, err
		}
	case *scalablePod.Status.Status == scalablev1.SPInactive:
		log.Printf("State: %s\n", scalablev1.SPInactive)
		if scalablePod.Status.Requested { // If someone has requested a ScalablePod, spin one up
			err := r.startAndBindPodTo(&scalablePod, ctx)
			if err != nil {
				log.Println("Unable to bind new pod to ScalablePod")
				return ctrl.Result{Requeue: true}, err
			}
			log.Printf("ScalablePod Pod namespaced name after startAndBindPodTo: `%s/%s`", scalablePod.Status.BoundPod.Namespace, scalablePod.Status.BoundPod.Name)
			*scalablePod.Status.Status = scalablev1.SPActive
			if err = r.Status().Update(ctx, &scalablePod); err != nil {
				log.Println("Unable to update ScalablePod status")
				return ctrl.Result{Requeue: true}, err
			}
			shutdownDuration, _ := time.ParseDuration(fmt.Sprintf("%ds", scalablePod.Spec.MaxActiveTimeSec))
			return ctrl.Result{RequeueAfter: shutdownDuration}, nil
		}
	case *scalablePod.Status.Status == scalablev1.SPActive:
		log.Printf("State: %s\n", scalablev1.SPActive)
		shutdownDuration, _ := time.ParseDuration(fmt.Sprintf("%ds", scalablePod.Spec.MaxActiveTimeSec))
		shutdownTime := scalablePod.Status.StartedAt.Add(shutdownDuration)
		if shutdownTime.Before(metav1.Now().Time) { // We need to spin down this ScalablePod
			err := r.deleteBoundPod(&scalablePod, ctx)
			if err != nil {
				log.Println("Unable to delete bound pod")
				return ctrl.Result{Requeue: true}, err
			}
			scalablePod.Status.Requested = false
			*scalablePod.Status.Status = scalablev1.SPInactive
			log.Printf("Changing status to %s\n", scalablev1.SPInactive)
			if err = r.Status().Update(ctx, &scalablePod); err != nil {
				log.Println("Unable to update ScalablePod status")
				return ctrl.Result{Requeue: true}, err
			}
		}
	}
	// Don't requeue
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalablePodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scalablev1.ScalablePod{}).
		// Owns(&corev1.Pod{}).
		Complete(r)
}

func (r *ScalablePodReconciler) deleteBoundPod(scalablePod *scalablev1.ScalablePod, ctx context.Context) error {
	var pod corev1.Pod
	r.Get(ctx, types.NamespacedName{Namespace: scalablePod.Status.BoundPod.Namespace, Name: scalablePod.Status.BoundPod.Name}, &pod)
	log.Printf("Removing bound pod w/name `%s` from inactive ScalablePod\n", pod.Name)
	err := r.Client.Delete(ctx, pod.DeepCopy())
	if err != nil {
		return err
	}
	scalablePod.Status.BoundPod = nil
	return nil
}

func (r *ScalablePodReconciler) startAndBindPodTo(scalablePod *scalablev1.ScalablePod, ctx context.Context) error {
	podName := uuid.New().String()
	pod := corev1.Pod{
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
	// Create the Pod
	log.Printf("Creating pod `%s`\n", pod.Name)
	if err := r.Client.Create(ctx, &pod); err != nil {
		log.Printf("Failed to create pod for requested ScalablePod `%s/%s`\n", scalablePod.Namespace, scalablePod.Name)
		return err
	}
	scalablePod.Status.BoundPod = &scalablev1.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
	scalablePod.Status.StartedAt = metav1.Now()
	return nil
}
