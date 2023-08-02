package controller

import (
	"fmt"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MachineReconciler) createAWSMachine() (ctrl.Result, error) {
	fmt.Println("Create AWS Machine")
	return ctrl.Result{}, nil
}
