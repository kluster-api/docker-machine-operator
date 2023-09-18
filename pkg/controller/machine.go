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
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/appscode/go/crypto/rand"
	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"
	core "k8s.io/api/core/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
	cutil "kmodules.xyz/client-go/conditions"
)

func (r *MachineReconciler) createMachine() error {
	if cutil.IsConditionTrue(r.machineObj.Status.Conditions, api.MachineConditionMachineCreating) ||
		cutil.IsConditionTrue(r.machineObj.Status.Conditions, string(api.MachineConditionTypeMachineReady)) {
		return nil
	}

	r.log.Info("Creating Machine", "Cloud", r.machineObj.Spec.Driver)
	err := r.createPrerequisitesForMachine()
	if err != nil {
		return err
	}
	args, err := r.getMachineCreationArgs()
	if err != nil {
		return err
	}

	cutil.MarkTrue(r.machineObj, api.MachineConditionMachineCreating)

	cmd := exec.Command("docker-machine", args...)
	var commandOutput, commandError bytes.Buffer
	cmd.Stdout = &commandOutput
	cmd.Stderr = &commandError

	err = cmd.Run()
	if err != nil && !strings.Contains(commandError.String(), "already exists") {
		r.log.Info("Error creating docker machine", "Error: ", commandError.String(), "Output: ", commandOutput.String())
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeMachineReady, err.Error(), kmapi.ConditionSeverityError,
			"Unable to create docker machine")

		return err
	}

	cutil.MarkTrue(r.machineObj, api.MachineConditionTypeMachineReady)
	r.log.Info("Created Docker Machine Successfully")
	return nil
}

func (r *MachineReconciler) createPrerequisitesForMachine() error {
	var err error = nil
	if r.machineObj.Spec.Driver.Name == AWSDriver {
		err = r.createAWSEnvironment()
	}
	return err
}

func (r *MachineReconciler) getMachineCreationArgs() ([]string, error) {
	var args []string
	args = append(args, "create", "--driver", r.machineObj.Spec.Driver.Name)

	for k, v := range r.machineObj.Spec.Parameters {
		args = append(args, fmt.Sprintf("--%s", k))
		args = append(args, v)
	}

	scriptArgs, err := r.getStartupScriptArgs()
	if err != nil {
		r.log.Error(err, "unable to create script")
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeScriptReady, err.Error(), kmapi.ConditionSeverityError, "unable to create script")
		return nil, err
	}
	cutil.MarkTrue(r.machineObj, api.MachineConditionTypeScriptReady)
	args = append(args, scriptArgs...)

	authArgs, err := r.getAuthSecretArgs()
	if err != nil {
		r.log.Error(err, "unable to read auth data")
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeAuthDataReady, err.Error(), kmapi.ConditionSeverityError, "unable to read auth data")
		return nil, err
	}
	cutil.MarkTrue(r.machineObj, api.MachineConditionTypeAuthDataReady)
	args = append(args, authArgs...)

	args = append(args, r.getAnnotationsArgs()...)

	args = append(args, r.getMachineUserArg()...)

	args = append(args, r.machineObj.Name)
	return args, nil
}

func (r *MachineReconciler) getMachineUserArg() []string {
	var userArgs []string
	driverName := r.machineObj.Spec.Driver.Name
	username := defaultUserName
	switch driverName {
	case GoogleDriver:
		userArgs = append(userArgs, "--google-username")
	case AWSDriver:
		userArgs = append(userArgs, "--amazonec2-ssh-user")
		username = defaultAWSUserName
	case AzureDriver:
		userArgs = append(userArgs, "--azure-ssh-user")
	}
	userArgs = append(userArgs, username)
	return userArgs
}

func (r *MachineReconciler) getAuthSecretArgs() ([]string, error) {
	authSecret, err := r.getSecret(r.machineObj.Spec.AuthSecret)
	if err != nil {
		return nil, err
	}
	var authArgs []string
	for key, value := range authSecret.Data {
		data := string(value)
		if r.machineObj.Spec.Driver.Name == GoogleDriver {
			data = base64.StdEncoding.EncodeToString(value)
		}
		if len(data) == 0 || len(key) == 0 {
			return nil, fmt.Errorf("auth secret not found")
		}
		authArgs = append(authArgs, fmt.Sprintf("--%s", key))
		authArgs = append(authArgs, data)
	}

	return authArgs, nil
}

func (r MachineReconciler) getAnnotationsArgs() []string {
	var annotationArgs []string
	if r.machineObj.Spec.Driver.Name == AWSDriver {
		if r.machineObj.Annotations[awsVPCIDAnnotation] != "" {
			annotationArgs = append(annotationArgs, "--amazonec2-vpc-id")
			annotationArgs = append(annotationArgs, r.machineObj.Annotations[awsVPCIDAnnotation])
		}
		if r.machineObj.Annotations[awsSubnetIDAnnotation] != "" {
			annotationArgs = append(annotationArgs, "--amazonec2-subnet-id")
			annotationArgs = append(annotationArgs, r.machineObj.Annotations[awsSubnetIDAnnotation])
		}
	}
	return annotationArgs
}

func (r *MachineReconciler) getStartupScriptArgs() ([]string, error) {
	scriptSecret, err := r.getSecret(r.machineObj.Spec.ScriptRef)
	if err != nil {
		return nil, err
	}
	var fileName string
	if r.scriptFileName != "" {
		fileName = r.scriptFileName
	} else {
		fileName = fmt.Sprintf("%s.sh", rand.WithUniqSuffix("script"))
	}

	filePath := tempDirectory + fileName

	var userDataKey, userDataValue string
	for key, value := range scriptSecret.Data {
		userDataKey = key
		userDataValue = string(value)
		if len(userDataKey) > 0 {
			break
		}
	}
	if len(userDataKey) == 0 || len(userDataValue) == 0 {
		return nil, fmt.Errorf("script data not found")
	}
	scriptArgs := []string{fmt.Sprintf("--%s", userDataKey)}
	scriptArgs = append(scriptArgs, filePath)

	_, err = os.Stat(filePath)
	if err == nil {
		return scriptArgs, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	err = os.WriteFile(filePath, []byte(userDataValue), 0644)
	if err != nil {
		return nil, err
	}
	r.scriptFileName = fileName
	return scriptArgs, nil
}

func (r *MachineReconciler) getSecret(secretRef *kmapi.ObjectReference) (core.Secret, error) {
	var secret core.Secret
	err := r.Client.Get(context.TODO(), secretRef.ObjectKey(), &secret)
	if err != nil {
		return core.Secret{}, err
	}
	return secret, nil
}
