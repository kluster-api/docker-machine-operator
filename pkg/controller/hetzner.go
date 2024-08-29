package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *MachineReconciler) createJob() error {
	configSecret := corev1.Secret{}
	kc := r.KBClient
	if err := kc.Get(r.ctx, types.NamespacedName{Name: r.machineObj.Spec.ScriptRef.Name, Namespace: r.machineObj.Spec.ScriptRef.Namespace}, &configSecret, &client.GetOptions{}); err != nil {
		return err
	}
	script := string(configSecret.Data["hetzner.sh"])
	klog.Info("script: ", script)
	klog.Info("..............Found the Script For Job.............")
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jb-" + r.machineObj.Spec.ScriptRef.Name,
			Namespace: r.machineObj.Spec.ScriptRef.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "capi-script",
							Image: "debian:12",
							Command: []string{
								"/etc/capi-script/hetzner.sh",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "script",
									ReadOnly:  true,
									MountPath: "/etc/capi-script",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "script",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  r.machineObj.Spec.ScriptRef.Name,
									DefaultMode: ptr.To(int32(0o755)),
								},
							},
						},
					},
				},
			},
			BackoffLimit: int32Ptr(0),
		},
	}
	err := kc.Create(r.ctx, job)
	if err != nil {
		return nil
	}
	klog.Info("...............Job Created.............")

	return nil
}

// Helper functions to get pointers to primitives
func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

func (r *MachineReconciler) isJobScriptFinished(jobName, namespace string) (bool, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := checkJobStatus(clientset, namespace, jobName)
			if err != nil {
				fmt.Printf("Error checking job status: %v\n", err)
				return false, err
			}

			if status == "success" {
				return true, nil
			} else if status == "failed" {
				return false, nil
			}
		}
	}
}

func checkJobStatus(clientset *kubernetes.Clientset, namespace, jobName string) (string, error) {
	job, err := clientset.BatchV1().Jobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if job.Status.Succeeded > 0 {
		return "success", nil
	} else if job.Status.Failed > 0 {
		return "failed", nil
	}

	return "running", nil
}

func int32Ptr(i int32) *int32 {
	return &i
}
