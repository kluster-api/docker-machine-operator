package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/appscode/go/crypto/rand"
	"github.com/pkg/errors"
	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"
	core "k8s.io/api/core/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
	cutil "kmodules.xyz/client-go/conditions"
	"os"
	"os/exec"
)

const scriptFileDirectory = "/tmp/"

func (r *MachineReconciler) createMachine(driverName string) error {
	if cutil.IsConditionTrue(r.machineObj.Status.Conditions, api.MachineConditionMachineCreating) {
		return nil
	}
	r.log.Info("Creating Google Compute Engine")
	args, err := r.getMachineCreationArgs(driverName)
	if err != nil {
		return err
	}
	cutil.MarkTrue(r.machineObj, api.MachineConditionMachineCreating)
	cmd := exec.Command("docker-machine", args...)
	fmt.Println(args)
	err = cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
		cutil.MarkFalse(r.machineObj, api.MachineConditionTypeMachineReady, err.Error(), kmapi.ConditionSeverityError,
			"Unable to create docker machine")
		return err
	}
	cutil.MarkTrue(r.machineObj, api.MachineConditionTypeMachineReady)
	r.log.Info("Created Docker Machine Successfully")
	return nil
}

func (r *MachineReconciler) getMachineCreationArgs(driverName string) ([]string, error) {
	var args []string
	args = append(args, "create", "--driver", driverName)

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
	args = append(args, r.machineObj.Name)
	return args, nil
}

func (r *MachineReconciler) getAuthSecretArgs() ([]string, error) {
	authSecret, err := r.getSecret(r.machineObj.Spec.AuthSecret)
	if err != nil {
		return nil, err
	}
	var authArgs []string
	for key, value := range authSecret.Data {
		data := base64.StdEncoding.EncodeToString(value)
		if len(data) == 0 || len(key) == 0 {
			cutil.MarkTrue(r.machineObj, api.MachineConditionAuthDataNotFound)
			return nil, fmt.Errorf("auth secret not found")
		}
		authArgs = append(authArgs, fmt.Sprintf("--%s", key))
		authArgs = append(authArgs, data)
	}

	return authArgs, nil
}

func (r *MachineReconciler) getStartupScriptArgs() ([]string, error) {
	scriptScret, err := r.getSecret(r.machineObj.Spec.ScriptRef)
	if err != nil {
		return nil, err
	}
	var fileName string
	if _, ok := r.machineObj.Annotations["script-file"]; ok {
		fileName = r.machineObj.Annotations["script-file"]
	} else {
		fileName = fmt.Sprintf("%s.sh", rand.WithUniqSuffix("script"))
	}

	filePath := scriptFileDirectory + fileName

	var userDataKey, userDataValue string
	for key, value := range scriptScret.Data {
		userDataKey = key
		userDataValue = string(value)
		if len(userDataKey) > 0 {
			break
		}
	}
	if len(userDataKey) == 0 || len(userDataValue) == 0 {
		cutil.MarkTrue(r.machineObj, api.MachineConditionScriptDataNotFound)
		return nil, fmt.Errorf("script data not found")
	}
	scriptArgs := []string{fmt.Sprintf("--%s", userDataKey)}
	scriptArgs = append(scriptArgs, filePath)

	_, err = os.Stat(filePath)
	if err == nil {
		return scriptArgs, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = os.WriteFile(filePath, []byte(userDataValue), 0644)
	if err != nil {
		return nil, err
	}

	r.machineObj.Annotations["script-file"] = fileName

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
