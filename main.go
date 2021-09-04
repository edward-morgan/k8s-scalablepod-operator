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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	scalablev1 "github.com/edwmorgan/k8s-operator-example/api/v1"
	"github.com/edwmorgan/k8s-operator-example/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(scalablev1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "7bc586a9.scalablepod.tutorial.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := &controllers.ScalablePodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ScalablePod")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	http.HandleFunc("/", RequestWrapper(reconciler))
	go http.ListenAndServe("localhost:19090", nil)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

/* When the reconciler receives an HTTP request to schedule a ScalablePod, this function handles the process of
 * choosing which ScalablePod should be activated.
 */
//TODO: Reconciler isn't needed, just a Client
func RequestWrapper(reconciler *controllers.ScalablePodReconciler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scalablePods := &scalablev1.ScalablePodList{}
		reconciler.Client.List(r.Context(), scalablePods)
		fmt.Println("Found " + fmt.Sprintf("%d", len(scalablePods.Items)) + " ScalablePods.")
		// Use round-robin scheduling to spin up a new ScalablePod
		for _, sp := range scalablePods.Items {
			if sp.Status.Status != nil && *sp.Status.Status == scalablev1.SPInactive {
				log.Printf("Found suitable Inactive ScalablePod with name: `%s` \n", sp.Name)
				sp.Status.Requested = true
				if err := reconciler.Status().Update(context.Background(), &sp); err != nil {
					// TODO: Implement additional status codes (ex. if no ScalablePods are currently available)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusAccepted)
				return
			}
		}
		// If no resources are available, return a 400
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("All resources in use. Try again later.\n"))
	}
}
