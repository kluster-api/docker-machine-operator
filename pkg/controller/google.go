package controller

import (
	"context"
	"fmt"
	core "k8s.io/api/core/v1"
	"os"
	"os/exec"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MachineReconciler) createGoogleMachine() (ctrl.Result, error) {
	r.log.Info("Creating Google Compute Engine")
	err := r.createScriptFile()
	if err != nil {
		return ctrl.Result{}, err
	}
	//cmd := []string{"docker-machine", "create"}
	var cmd []string
	driverName := r.machineObj.Spec.DriverRef.Name
	cmd = append(cmd, "create", "--driver", driverName)

	for k, v := range r.machineObj.Spec.Parameters {
		cmd = append(cmd, fmt.Sprintf("--%s", k))
		cmd = append(cmd, v)
	}
	//cmd = append(cmd, "--google-auth-encoded-json", "")
	cmd = append(cmd, "--google-user-data", "/tmp/gcp.sh")

	cmd = append(cmd, r.machineObj.Name)

	fmt.Println(cmd)

	command := exec.Command("docker-machine", cmd...)
	err = command.Run()
	if err != nil {
		fmt.Println("Could not create machine")
		fmt.Println(err.Error())
	}

	fmt.Println("Machine created successfully")

	return ctrl.Result{}, nil
}

func (r *MachineReconciler) createScriptFile() error {
	var secret core.Secret
	scriptRef := r.machineObj.Spec.ScriptRef
	err := r.Client.Get(context.TODO(), scriptRef.ObjectKey(), &secret)
	if err != nil {
		return err
	}
	data := string(secret.Data["gcp"])

	err = os.WriteFile("/tmp/gcp.sh", []byte(data), 0644)
	if err != nil {
		return err
	}

	return nil
}
