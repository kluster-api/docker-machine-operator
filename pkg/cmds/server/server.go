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

package server

import (
	"context"
	"flag"
	"os"
	"time"

	dockermachinev1alpha1 "go.klusters.dev/docker-machine-operator/api/v1alpha1"
	"go.klusters.dev/docker-machine-operator/pkg/controller"

	"github.com/spf13/pflag"
	v "gomodules.xyz/x/version"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	cu "kmodules.xyz/client-go/client"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	setupLog = log.Log.WithName("setup")
	scheme   = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dockermachinev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type OperatorOptions struct {
	MasterURL      string
	KubeconfigPath string
	LicenseFile    string
	QPS            float64
	Burst          int
	ResyncPeriod   time.Duration
	MaxNumRequeues int
	NumThreads     int

	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
}

func NewOperatorOptions() *OperatorOptions {
	return &OperatorOptions{
		ResyncPeriod:   10 * time.Minute,
		MaxNumRequeues: 5,
		NumThreads:     2,
		// ref: https://github.com/kubernetes/ingress-nginx/blob/e4d53786e771cc6bdd55f180674b79f5b692e552/pkg/ingress/controller/launch.go#L252-L259
		// High enough QPS to fit all expected use cases. QPS=0 is not set here, because client code is overriding it.
		QPS: 1e6,
		// High enough Burst to fit all expected use cases. Burst=0 is not set here, because client code is overriding it.
		Burst:                1e6,
		metricsAddr:          ":8080",
		enableLeaderElection: false,
		probeAddr:            ":8081",
	}
}

func (s *OperatorOptions) AddFlags(fs *pflag.FlagSet) {
	pfs := flag.NewFlagSet("extra-flags", flag.ExitOnError)
	s.AddGoFlags(pfs)
	fs.AddGoFlagSet(pfs)
}

func (s *OperatorOptions) AddGoFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.MasterURL, "master", s.MasterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.KubeconfigPath, "kubeconfig", s.KubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")

	fs.StringVar(&s.LicenseFile, "license-file", s.LicenseFile, "Path to license file")

	fs.Float64Var(&s.QPS, "qps", s.QPS, "The maximum QPS to the master from this client")
	fs.IntVar(&s.Burst, "burst", s.Burst, "The maximum burst for throttle")
	fs.DurationVar(&s.ResyncPeriod, "resync-period", s.ResyncPeriod, "If non-zero, will re-list this often. Otherwise, re-list will be delayed aslong as possible (until the upstream source closes the watch or times out.")

	fs.StringVar(&s.metricsAddr, "metrics-bind-address", s.metricsAddr, "The address the metric endpoint binds to.")
	fs.StringVar(&s.probeAddr, "health-probe-bind-address", s.probeAddr, "The address the probe endpoint binds to.")
	fs.BoolVar(&s.enableLeaderElection, "leader-elect", s.enableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
}

func (s OperatorOptions) Run(ctx context.Context) error {
	klog.Infof("Starting binary version %s+%s ...", v.Version.Version, v.Version.CommitHash)

	ctrl.SetLogger(klogr.New()) // nolint:staticcheck

	cfg, err := clientcmd.BuildConfigFromFlags(s.MasterURL, s.KubeconfigPath)
	if err != nil {
		klog.Fatalf("Could not get Kubernetes config: %s", err)
	}

	cfg.QPS = float32(s.QPS)
	cfg.Burst = s.Burst

	syncPeriod := time.Second * 60

	mgr, err := manager.New(cfg, manager.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: ""},
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		HealthProbeBindAddress: s.probeAddr,
		LeaderElection:         s.enableLeaderElection,
		LeaderElectionID:       "54995429.klusters.dev",
		NewClient:              cu.NewClient,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.DriverReconciler{
		KBClient: mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Driver")
		os.Exit(1)
	}
	if err = (&controller.MachineReconciler{
		KBClient: mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Machine")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil
}
