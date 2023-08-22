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
	"fmt"
	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"
	cutil "kmodules.xyz/client-go/conditions"
	"os"
	"os/exec"
	"strconv"
)

func (r *MachineReconciler) isScriptFinished() error {
	args := r.getScpArgs()
	fmt.Println(args)
	cmd := exec.Command("docker-machine", args...)
	err := cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
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
				cutil.MarkTrue(r.machineObj, api.MachineConditionClusterCreatedSuccessfully)
				return nil
			}
		} else {
			fmt.Println(err.Error())
		}
	}

	return fmt.Errorf("failed to crate cluster")
}

func (r *MachineReconciler) getScpArgs() []string {
	var args = []string{"scp"}
	userName := "docker-user"
	if v, found := r.machineObj.Spec.Parameters["google-username"]; found {
		userName = v
	}
	machineName := r.machineObj.Name
	args = append(args, fmt.Sprintf("%s@%s:/tmp/result.txt", userName, machineName))
	args = append(args, "/tmp")

	return args
}
