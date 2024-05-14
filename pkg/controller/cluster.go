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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"

	kmapi "kmodules.xyz/client-go/api/v1"
	cutil "kmodules.xyz/client-go/conditions"
)

const resultFile = "/tmp/result.txt"

func (r *MachineReconciler) isScriptFinished() (bool, error) {
	if !cutil.IsConditionTrue(r.machineObj.Status.Conditions, string(api.MachineConditionTypeMachineReady)) {
		return false, nil
	}
	if cutil.IsConditionTrue(r.machineObj.Status.Conditions, string(api.MachinePhaseSuccess)) || cutil.IsConditionTrue(r.machineObj.Status.Conditions, string(api.MachinePhaseFailed)) {
		return false, nil
	}

	args := r.getScpArgs()
	cmd := exec.Command("docker-machine", args...)
	var commandOutput, commandError bytes.Buffer
	cmd.Stdout = &commandOutput
	cmd.Stderr = &commandError

	err := cmd.Run()
	if err != nil {
		r.Log.Info("Waiting for Script Completion. Checking Again in 1 minute. ", "CommandError: ", commandError.String(), "Output: ", commandOutput.String(), "Error: ", err.Error())
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeClusterOperationComplete, api.ReasonWaitingForScriptCompletion, kmapi.ConditionSeverityError, "waiting for script completion")
		return true, nil
	}
	r.Log.Info("Finished Cluster Operation Script.")

	file, err := os.Open(resultFile)
	if err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		resStr := scanner.Text()
		ret, err := strconv.Atoi(resStr)
		if err == nil {
			var createError error = nil
			if ret == 0 {
				r.Log.Info("Cluster Operation Finished Successfully")
				cutil.MarkTrue(r.machineObj, api.MachineConditionTypeClusterOperationComplete)
			} else {
				r.Log.Info("Cluster Operation Failed")
				cutil.MarkFalse(r.machineObj, api.MachineConditionTypeClusterOperationComplete, api.ReasonClusterOperationFailed, kmapi.ConditionSeverityError, "failed to create cluster")
				createError = fmt.Errorf("failed to create cluster")
			}
			err := os.Remove(resultFile)
			if err != nil {
				return false, err
			}
			return false, createError

		} else {
			r.Log.Info("Failed to Check Script Completion", "Error: ", err.Error())
		}
	}
	return false, fmt.Errorf("failed to create cluster")
}

func (r *MachineReconciler) getScpArgs() []string {
	args := []string{"scp"}
	machineName := r.machineObj.Name

	args = append(args, fmt.Sprintf("%s@%s:/tmp/result.txt", r.getDefaultUser(), machineName))
	args = append(args, "/tmp")

	return args
}

func (r *MachineReconciler) getDefaultUser() string {
	var defaultUser string
	driverName := r.machineObj.Spec.Driver.Name
	switch driverName {
	case GoogleDriver:
		defaultUser = "docker-user"
	case AWSDriver:
		defaultUser = "ubuntu"
	case AzureDriver:
		defaultUser = "docker-user"
	}
	return defaultUser
}
