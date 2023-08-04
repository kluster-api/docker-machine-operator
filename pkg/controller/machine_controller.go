/*
Copyright 2023.

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
	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dockermachinev1alpha1 "go.klusters.dev/docker-machine-operator/api/v1alpha1"
)

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	client.Client
	log        logr.Logger
	machineObj *dockermachinev1alpha1.Machine
	Scheme     *runtime.Scheme
}

//+kubebuilder:rbac:groups=docker-machine.klusters.dev,resources=machines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=docker-machine.klusters.dev,resources=machines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=docker-machine.klusters.dev,resources=machines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Machine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *MachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	var machine dockermachinev1alpha1.Machine
	if err := r.Get(ctx, req.NamespacedName, &machine); err != nil {
		r.log.Error(err, "unable to fetch machine object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.machineObj = machine.DeepCopy()

	driver := r.machineObj.Spec.Driver.Name
	switch driver {
	case GoogleDriver:
		return r.createGoogleMachine()
	case AWSDriver:
		return r.createAWSMachine()
	case AzureDriver:
		return r.createAzureMachine()
	default:
		r.log.Info("No driver found ", "driver name", driver)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dockermachinev1alpha1.Machine{}).
		Complete(r)
}
