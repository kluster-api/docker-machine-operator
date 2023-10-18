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
	kutil "kmodules.xyz/client-go"
	cu "kmodules.xyz/client-go/client"
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

func (r *MachineReconciler) processFinalizer(ctx context.Context) (bool, error) {
	if r.machineObj.DeletionTimestamp.IsZero() {
		err := r.ensureFinalizer()
		if err != nil {
			return false, err
		}
	} else {
		// Machine Object is Deleted
		return false, r.removeFinalizerAfterCleanup(ctx)
	}
	return true, nil
}

func (r *MachineReconciler) ensureFinalizer() error {
	finalizerName := api.GetFinalizer()
	if !controllerutil.ContainsFinalizer(r.machineObj, finalizerName) {
		if err := r.patchFinalizer(kutil.VerbCreated, finalizerName); err != nil {
			return err
		}
		r.log.Info(fmt.Sprintf("Finalizer %v added", finalizerName))
	}

	return nil
}

func (r *MachineReconciler) removeFinalizerAfterCleanup(ctx context.Context) error {
	finalizerName := api.GetFinalizer()
	if controllerutil.ContainsFinalizer(r.machineObj, finalizerName) {
		if err := r.cleanupMachineResources(ctx); err != nil {
			return err
		}
		if err := r.patchFinalizer(kutil.VerbDeleted, finalizerName); err != nil {
			return err
		}
		r.log.Info(fmt.Sprintf("Finalizer %v removed", finalizerName))
	}
	return nil
}

func (r *MachineReconciler) patchFinalizer(verbType kutil.VerbType, finalizerName string) error {
	_, err := cu.CreateOrPatch(context.TODO(), r.Client, r.machineObj, func(object client.Object, createOp bool) client.Object {
		if verbType == kutil.VerbCreated {
			controllerutil.AddFinalizer(object, finalizerName)
		} else if verbType == kutil.VerbDeleted {
			controllerutil.RemoveFinalizer(object, finalizerName)
		}
		return object
	})
	return err
}

func (r *MachineReconciler) cleanupMachineResources(ctx context.Context) error {
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
		err = r.cleanupAWSResources(ctx)
		if err != nil {
			return err
		}

	} else if r.machineObj.Spec.Driver.Name == AzureDriver {
		err = r.deleteAzureResourceGroup(ctx)
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
		r.log.Info("Error machine deletion", "Error: ", commandError.String(), "Output: ", commandOutput.String())
		return client.IgnoreNotFound(err)
	}
	return nil
}

func (r *MachineReconciler) patchAnnotation(key, value string) error {
	_, err := cu.CreateOrPatch(context.TODO(), r.Client, r.machineObj, func(object client.Object, createOp bool) client.Object {
		anno := object.GetAnnotations()
		if anno == nil {
			anno = make(map[string]string)
		}
		anno[key] = value
		object.SetAnnotations(anno)
		return object
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
