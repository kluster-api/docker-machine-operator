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

func (r *MachineReconciler) isScriptFinished() error {
	if !cutil.IsConditionTrue(r.machineObj.Status.Conditions, string(api.MachineConditionTypeMachineReady)) {
		return nil
	}

	args := r.getScpArgs()
	cmd := exec.Command("docker-machine", args...)
	var commandOutput, commandError bytes.Buffer
	cmd.Stdout = &commandOutput
	cmd.Stderr = &commandError

	err := cmd.Run()
	if err != nil {
		r.log.Info("Error checking script completion", "Error: ", commandError.String(), "Output: ", commandOutput.String())
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeClusterReady, api.MachineConditionWaitingForScriptCompletion, kmapi.ConditionSeverityError, "failed to check script completion")
		return err
	}
	r.log.Info("Finished Cluster Creation Script.")

	file, err := os.Open("/tmp/result.txt")
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		resStr := scanner.Text()
		ret, err := strconv.Atoi(resStr)
		if err == nil {
			r.log.Info("Script return code: " + strconv.Itoa(ret))
			if ret == 0 {
				cutil.MarkTrue(r.machineObj, api.MachineConditionTypeClusterReady)
				return nil
			}
		} else {
			r.log.Info("Failed to create cluster", "Error: ", err.Error())
		}
	}
	cutil.MarkFalse(r.machineObj, api.MachineConditionTypeClusterReady, api.MachineConditionClusterCreateFailed, kmapi.ConditionSeverityError, "failed to create cluster")

	return fmt.Errorf("failed to create cluster")
}

func (r *MachineReconciler) getScpArgs() []string {
	var args = []string{"scp"}
	machineName := r.machineObj.Name
	args = append(args, fmt.Sprintf("%s@%s:/tmp/result.txt", defaultUserName, machineName))
	args = append(args, "/tmp")

	return args
}
