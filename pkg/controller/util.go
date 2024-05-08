/*
Copyright AppsCode Inc. and Contributors.

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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	kutil "kmodules.xyz/client-go"
	kmapi "kmodules.xyz/client-go/api/v1"
	cu "kmodules.xyz/client-go/client"
	cutil "kmodules.xyz/client-go/conditions"
	coreutil "kmodules.xyz/client-go/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	GoogleDriver string = "google"
	AWSDriver    string = "amazonec2"
	AzureDriver  string = "azure"
)

const (
	defaultUserName    = "docker-user"
	defaultAWSUserName = "ubuntu"
	tempDirectory      = "tmp"
)

func (r *MachineReconciler) ensureFinalizer() error {
	finalizerName := api.GetFinalizer()
	if !controllerutil.ContainsFinalizer(r.machineObj, finalizerName) {
		if err := r.patchFinalizer(kutil.VerbCreated, finalizerName); err != nil {
			return err
		}
		r.Log.Info(fmt.Sprintf("Finalizer %v added", finalizerName))
	}

	return nil
}

func (r *MachineReconciler) removeFinalizerAfterCleanup() error {
	finalizerName := api.GetFinalizer()
	if controllerutil.ContainsFinalizer(r.machineObj, finalizerName) {
		if err := r.updateMachineStatus(types.NamespacedName{Namespace: r.machineObj.Namespace, Name: r.machineObj.Name}); err != nil {
			return err
		}
		if err := r.cleanupMachineResources(); err != nil {
			return err
		}
		if err := r.patchFinalizer(kutil.VerbDeleted, finalizerName); err != nil {
			return err
		}
		r.Log.Info(fmt.Sprintf("Finalizer %v removed", finalizerName))
	}
	return nil
}

func (r *MachineReconciler) patchFinalizer(verbType kutil.VerbType, finalizerName string) error {
	_, err := cu.CreateOrPatch(context.TODO(), r.KBClient, r.machineObj, func(object client.Object, createOp bool) client.Object {
		mc := object.(*api.Machine)
		if verbType == kutil.VerbCreated {
			mc.ObjectMeta = coreutil.AddFinalizer(mc.ObjectMeta, finalizerName)
		} else if verbType == kutil.VerbDeleted {
			mc.ObjectMeta = coreutil.RemoveFinalizer(mc.ObjectMeta, finalizerName)
		}
		return mc
	})
	return err
}

func (r *MachineReconciler) cleanupMachineResources() error {
	var err error
	err = r.deleteFiles()
	if err != nil {
		return err
	}
	err = r.deleteDockerMachine()
	if err != nil {
		return err
	}
	if r.machineObj.Spec.Driver.Name == AWSDriver {
		err = r.cleanupAWSResources()
		if err != nil {
			return err
		}

	} else if r.machineObj.Spec.Driver.Name == AzureDriver {
		err = r.deleteAzureResourceGroup()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *MachineReconciler) deleteFiles() error {
	err := os.Remove(r.getScriptFilePath())
	if err != nil && os.IsExist(err) {
		return err
	}

	resultFilePath := tempDirectory + "result.txt"
	err = os.Remove(resultFilePath)
	if err != nil && os.IsExist(err) {
		return err
	}
	return nil
}

func (r *MachineReconciler) deleteDockerMachine() error {
	args := []string{"rm", r.machineObj.Name, "-y"}
	cmd := exec.Command("docker-machine", args...)
	var commandOutput, commandError bytes.Buffer
	cmd.Stdout = &commandOutput
	cmd.Stderr = &commandError

	err := cmd.Run()
	if err != nil {
		r.Log.Info("Error machine deletion", "Error: ", commandError.String(), "Output: ", commandOutput.String())
		return client.IgnoreNotFound(err)
	}
	return nil
}

// reconciled returns an empty result with nil error to signal a successful reconcile
// to the controller manager
func (r *MachineReconciler) reconciled() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// requeueWithError is a wrapper around logging an error message
// then passes the error through to the controller manager
func (r *MachineReconciler) requeueWithError(msg string, err error) (ctrl.Result, error) {
	updErr := r.updateMachineStatus(types.NamespacedName{Namespace: r.machineObj.Namespace, Name: r.machineObj.Name})
	if updErr != nil {
		return ctrl.Result{}, updErr
	}
	// Info Log the error message and then let the reconciler dump the stacktrace
	r.Log.Info(msg, "Reason : ", err.Error())
	return ctrl.Result{}, err
}

func (r *MachineReconciler) updateMachineStatus(namespacedName client.ObjectKey) error {
	machine := &api.Machine{}
	if err := r.KBClient.Get(r.ctx, namespacedName, machine); err != nil {
		return err
	}
	cutil.SetSummary(r.machineObj, cutil.WithConditions(api.ConditionsOrder()...))
	r.machineObj.Status.Phase = api.GetPhase(r.machineObj)

	if err := r.committer(r.ctx, machine, r.machineObj); err != nil {
		return err
	}
	return nil
}

func (r *MachineReconciler) setInitialConditions() error {
	cutil.MarkFalse(r.machineObj, api.MachineConditionTypeMachineReady, api.ReasonMachineCreating, kmapi.ConditionSeverityError,
		"Waiting for Machine to become ready")
	cutil.MarkFalse(r.machineObj, api.MachineConditionTypeClusterOperationComplete, api.ReasonWaitingForScriptRun, kmapi.ConditionSeverityError,
		"Waiting for Script to run")
	err := r.updateMachineStatus(types.NamespacedName{Name: r.machineObj.Name, Namespace: r.machineObj.Namespace})
	if err != nil {
		return err
	}
	return nil
}

// isMarkedForDeletion determines if the object is marked for deletion
func (r *MachineReconciler) isMarkedForDeletion() bool {
	return !r.machineObj.GetDeletionTimestamp().IsZero()
}

func (r *MachineReconciler) SetLoggerWithReq(req ctrl.Request) {
	r.Log = ctrl.Log.WithValues(api.ResourceKindMachine, req.NamespacedName)
}

func (r *MachineReconciler) patchAnnotation(key, value string) error {
	_, err := cu.CreateOrPatch(context.TODO(), r.KBClient, r.machineObj, func(object client.Object, createOp bool) client.Object {
		mc := object.(*api.Machine)
		anno := mc.GetAnnotations()
		if anno == nil {
			anno = make(map[string]string)
		}
		anno[key] = value
		mc.SetAnnotations(anno)
		return mc
	})
	return err
}

func stringToP(st string) *string {
	return &st
}

func stringPSlice(sl []string) []*string {
	var ret []*string
	for i := 0; i < len(sl); i++ {
		ret = append(ret, &sl[i])
	}
	return ret
}

func waitForState(retry, timeout time.Duration, getStatus func() (bool, error)) error {
	for t := time.Second * 0; t <= timeout; t += retry {
		fmt.Println("getting state")
		res, err := getStatus()
		if err != nil {
			return err
		}
		if res {
			return nil
		}
		fmt.Println("retrying")
		time.Sleep(retry)
	}
	return fmt.Errorf("failed to get desired status")
}

func (r *MachineReconciler) getScriptFilePath() string {
	return fmt.Sprintf("/%s/%s-%s-startup.sh", tempDirectory, r.machineObj.Namespace, r.machineObj.Name)
}
