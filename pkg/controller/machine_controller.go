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
	"time"

	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"

	"github.com/go-logr/logr"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"kmodules.xyz/client-go/conditions/committer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	ctx        context.Context
	committer  func(ctx context.Context, old, obj committer.StatusGetter[*api.MachineStatus]) error
	KBClient   client.Client
	Log        logr.Logger
	machineObj *api.Machine
	Scheme     *runtime.Scheme
}

//+kubebuilder:rbac:groups=docker-machine.klusters.dev,resources=machines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=docker-machine.klusters.dev,resources=machines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=docker-machine.klusters.dev,resources=machines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Machine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *MachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Info("........Reconciling Machine........")
	r.Log = log.FromContext(ctx)

	r.committer = committer.NewStatusCommitter[*api.Machine, *api.MachineStatus](r.KBClient.Status())

	message, err := r.updateMachineReconcile(ctx, req.NamespacedName)
	if err != nil {
		if kerr.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return r.requeueWithError(message, err)
	}
	if r.machineObj.Status.Phase == "" {
		return ctrl.Result{}, r.updateMachineStatus(req.NamespacedName)
	}

	if r.isMarkedForDeletion() {
		if err := r.removeFinalizerAfterCleanup(); err != nil {
			klog.Errorln(err)
			return r.requeueWithError("", err)
		}
		return r.reconciled()
	}
	klog.Info()

	err = r.ensureFinalizer()
	if err != nil {
		if kerr.IsNotFound(err) {
			return r.reconciled()
		}
		return r.requeueWithError("Failed to ensure Finalizers", err)
	}
	reconcileResult := ctrl.Result{}
	rekey := false

	if r.machineObj.Spec.Parameters["provider"] == "hetzner" {
		err = r.createJob()
		if err != nil {
			return r.requeueWithError("Failed to Create Job", err)
		}
		rekey, err = r.isJobScriptFinished("jb-"+r.machineObj.Spec.ScriptRef.Name, r.machineObj.Spec.ScriptRef.Namespace)
		if rekey == false {
			return r.requeueWithError("", err)
		}

	} else {
		err = r.createMachine()
		if err != nil {
			return r.requeueWithError("Failed to create Machine", err)
		}

		rekey, err = r.isScriptFinished()
		if err != nil {
			return r.requeueWithError("", err)
		}

	}
	if rekey {
		reconcileResult.RequeueAfter = time.Minute * 1
	}
	return reconcileResult, r.updateMachineStatus(req.NamespacedName)
}

func (r *MachineReconciler) updateMachineReconcile(ctx context.Context, namespacedName client.ObjectKey) (string, error) {
	machine := &api.Machine{}
	if err := r.KBClient.Get(ctx, namespacedName, machine); err != nil {
		return "Failed to get Machine", err
	}
	r.machineObj = machine
	r.ctx = ctx

	return "", nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.Machine{}).
		Complete(r)
}
