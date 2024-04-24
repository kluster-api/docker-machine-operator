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
	"time"

	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kmapi "kmodules.xyz/client-go/api/v1"
	cutil "kmodules.xyz/client-go/conditions"
)

const machineCreationTimeout = 15 * time.Minute

func (r *MachineReconciler) createMachine(ctx context.Context) error {
	if cutil.IsConditionTrue(r.machineObj.Status.Conditions, api.MachineConditionMachineCreating) ||
		cutil.IsConditionTrue(r.machineObj.Status.Conditions, string(api.MachineConditionTypeMachineReady)) {
		return nil
	}

	err := r.createPrerequisitesForMachine(ctx)
	if err != nil {
		return err
	}
	args, err := r.getMachineCreationArgs(ctx)
	if err != nil {
		return err
	}
	r.log.Info("Creating Machine", "MachineName", r.machineObj.Name, "Driver", r.machineObj.Spec.Driver)

	newCtx, cancel := context.WithTimeout(ctx, machineCreationTimeout)
	defer cancel()

	cutil.MarkTrue(r.machineObj, api.MachineConditionMachineCreating)
	cmd := exec.CommandContext(newCtx, "docker-machine", args...)
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
	r.log.Info("Created Docker Machine Successfully", "MachineName", r.machineObj.Name, "Driver", r.machineObj.Spec.Driver)
	return nil
}

func (r *MachineReconciler) createPrerequisitesForMachine(ctx context.Context) error {
	if r.machineObj.Spec.Driver.Name == AWSDriver {
		return r.createAWSEnvironment(ctx)
	}
	return nil
}

func (r *MachineReconciler) getMachineCreationArgs(ctx context.Context) ([]string, error) {
	var args []string
	args = append(args, "create", "--driver", r.machineObj.Spec.Driver.Name)

	for k, v := range r.machineObj.Spec.Parameters {
		args = append(args, fmt.Sprintf("--%s", k))
		args = append(args, v)
	}

	if r.machineObj.Spec.ScriptRef != nil {
		scriptArgs, err := r.getStartupScriptArgs(ctx)
		if err != nil {
			cutil.MarkFalse(r.machineObj, api.MachineConditionTypeScriptReady, api.MachineConditionScriptDataNotFound, kmapi.ConditionSeverityError, "unable to create script")
			return nil, err
		}
		cutil.MarkTrue(r.machineObj, api.MachineConditionTypeScriptReady)
		args = append(args, scriptArgs...)
	}

	authArgs, err := r.getAuthSecretArgs(ctx)
	if err != nil {
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeAuthDataReady, api.MachineConditionAuthDataNotFound, kmapi.ConditionSeverityError, "unable to read auth data")
		return nil, err
	}
	cutil.MarkTrue(r.machineObj, api.MachineConditionTypeAuthDataReady)
	args = append(args, authArgs...)

	args = append(args, r.getAnnotationsArgsForAWS()...)
	args = append(args, r.machineObj.Name)

	return args, nil
}

func (r *MachineReconciler) getAuthSecretArgs(ctx context.Context) ([]string, error) {
	authSecret, err := r.getSecret(ctx, r.machineObj.Spec.AuthSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.Info("auth secret is not ready yet", "name", r.machineObj.Spec.AuthSecret)
		} else {
			r.log.Error(err, "error in auth secret", "name", r.machineObj.Spec.AuthSecret)
		}
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

func (r *MachineReconciler) getStartupScriptArgs(ctx context.Context) ([]string, error) {
	scriptSecret, err := r.getSecret(ctx, r.machineObj.Spec.ScriptRef)
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.Info("script secret is not ready yet", "name", r.machineObj.Spec.ScriptRef)
		} else {
			r.log.Error(err, "error in script secret", "name", r.machineObj.Spec.ScriptRef)
		}

		return nil, err
	}
	var filePath = r.getScriptFilePath()

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
	r.log.Info("writing start up script in file", "Filepath", filePath)

	err = os.WriteFile(filePath, []byte(userDataValue), 0644)
	if err != nil {
		return nil, err
	}
	return scriptArgs, nil
}

func (r *MachineReconciler) getSecret(ctx context.Context, secretRef *kmapi.ObjectReference) (core.Secret, error) {
	var secret core.Secret
	err := r.Client.Get(ctx, secretRef.ObjectKey(), &secret)
	return secret, err
}
