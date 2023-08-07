package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/appscode/go/crypto/rand"
	core "k8s.io/api/core/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
	"os"
	"os/exec"
)

const scriptFileDirectory = "/tmp/"

func (r *MachineReconciler) createMachine(driverName string) error {
	r.log.Info("Creating Google Compute Engine")
	args, err := r.getMachineCreationArgs(driverName)

	cmd := exec.Command("docker-machine", args...)
	err = cmd.Run()
	if err != nil {
		fmt.Println("Could not create machine")
		fmt.Println(err.Error())
		return err
	}
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
		return nil, err
	}
	args = append(args, scriptArgs...)

	authArgs, err := r.getAuthSecretArgs()
	if err != nil {
		return nil, err
	}
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

	fileName := fmt.Sprintf("%s.sh", rand.WithUniqSuffix("script"))
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
		return nil, fmt.Errorf("script data not found")
	}
	err = os.WriteFile(filePath, []byte(userDataValue), 0644)
	if err != nil {
		return nil, err
	}

	scriptArgs := []string{fmt.Sprintf("--%s", userDataKey)}
	scriptArgs = append(scriptArgs, filePath)
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
