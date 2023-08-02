package controller

import (
	"fmt"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *MachineReconciler) createGoogleMachine() (ctrl.Result, error) {
	fmt.Println("Create Google Machine")
	return ctrl.Result{}, nil
}
