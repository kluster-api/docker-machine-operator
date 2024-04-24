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

package cmds

import (
	"context"

	"github.com/spf13/cobra"
	api "go.klusters.dev/docker-machine-operator/api/v1alpha1"
	"go.klusters.dev/docker-machine-operator/pkg/cmds/server"
	v "gomodules.xyz/x/version"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(api.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func NewCmdRun(ctx context.Context) *cobra.Command {
	o := server.NewOperatorOptions()
	cmd := &cobra.Command{
		Use:               "run",
		Short:             "Launch Docker Machine operator",
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			klog.Infof("Starting binary version %s+%s ...", v.Version.Version, v.Version.CommitHash)

			return o.Run(ctx)
		},
	}

	o.AddFlags(cmd.Flags())

	return cmd
}
