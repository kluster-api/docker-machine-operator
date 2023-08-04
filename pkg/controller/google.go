package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	core "k8s.io/api/core/v1"
	"os"
	"os/exec"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MachineReconciler) createGoogleMachine() (ctrl.Result, error) {
	r.log.Info("Creating Google Compute Engine")
	scriptFile, err := r.createScriptFile()
	if err != nil {
		return ctrl.Result{}, err
	}
	//cmd := []string{"docker-machine", "create"}
	var cmd []string
	driverName := r.machineObj.Spec.Driver.Name
	cmd = append(cmd, "create", "--driver", driverName)

	for k, v := range r.machineObj.Spec.Parameters {
		cmd = append(cmd, fmt.Sprintf("--%s", k))
		cmd = append(cmd, v)
	}

	authData, err := r.getAuthSecret()
	if err != nil {
		return ctrl.Result{}, err
	}

	cmd = append(cmd, "--google-auth-encoded-json", authData)
	cmd = append(cmd, "--google-userdata", scriptFile)

	cmd = append(cmd, r.machineObj.Name)

	fmt.Println(cmd)

	command := exec.Command("docker-machine", cmd...)
	err = command.Run()
	if err != nil {
		fmt.Println("Could not create machine")
		fmt.Println(err.Error())
		return ctrl.Result{}, err
	}

	fmt.Println("Machine created successfully")

	return ctrl.Result{}, nil
}

func (r *MachineReconciler) getAuthSecret() (string, error) {
	var secret core.Secret
	secretRef := r.machineObj.Spec.AuthSecret
	err := r.Client.Get(context.TODO(), secretRef.ObjectKey(), &secret)
	if err != nil {
		return "", err
	}
	data := base64.StdEncoding.EncodeToString(secret.Data["cred"])
	if len(data) == 0 {
		return "", fmt.Errorf("auth secret not found")
	}
	return data, nil
}

func (r *MachineReconciler) createScriptFile() (string, error) {
	var secret core.Secret
	scriptRef := r.machineObj.Spec.ScriptRef
	err := r.Client.Get(context.TODO(), scriptRef.ObjectKey(), &secret)
	if err != nil {
		return "", err
	}
	data := string(secret.Data["gcp"])
	fileName := "/tmp/gcp.sh"

	err = os.WriteFile("/tmp/gcp.sh", []byte(data), 0644)
	if err != nil {
		return "", err
	}

	return fileName, nil
}
