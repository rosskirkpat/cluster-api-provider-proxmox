/*
Copyright 2023 Ross Kirkpatrick.

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

package main

import (
	"flag"
	"fmt"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"k8s.io/apimachinery/pkg/api/meta"
	"os"
	"reflect"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrastructurev1alpha1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	controller "github.com/rosskirkpat/cluster-api-provider-proxmox/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(infrastructurev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var err error
	options := ctrl.Options{Scheme: scheme}
	if configFile != "" {
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile))
		if err != nil {
			setupLog.Error(err, "unable to load the config file")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.ProxmoxClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxCluster")
		os.Exit(1)
	}
	if err = (&controller.ProxmoxMachineReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxMachine")
		os.Exit(1)
	}

	_ = func(ctx *context.ControllerContext, mgr ctrl.Manager) {
		cluster := &infrastructurev1alpha1.ProxmoxCluster{}
		gvr := infrastructurev1alpha1.GroupVersion.WithResource(reflect.TypeOf(cluster).Elem().Name())
		_, err := mgr.GetRESTMapper().KindFor(gvr)
		if err != nil {
			if meta.IsNoMatchError(err) {
				setupLog.Info(fmt.Sprintf("CRD for %s not loaded, skipping.", gvr.String()))
			} else {
				setupLog.Error(err, fmt.Sprintf("unexpected error while looking for CRD %s", gvr.String()), "crd", "ProxmoxCluster")
				os.Exit(1)
			}
		} else {
			if err := (&infrastructurev1alpha1.ProxmoxVM{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxVM")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxVMList{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxVMList")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxClusterTemplate")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxMachine{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxMachine")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxMachineTemplateWebhook{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxMachineTemplateWebhook")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxMachineTemplateList{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxMachineTemplateList")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxDeploymentZone{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxDeploymentZone")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxFailureDomain{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxFailureDomain")
				os.Exit(1)
			}
			if err := (&infrastructurev1alpha1.ProxmoxMachineList{}).SetupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create webhook", "webhook", "ProxmoxMachineList")
				os.Exit(1)
			}
			if err := controller.AddClusterControllerToManager(ctx, mgr, &infrastructurev1alpha1.ProxmoxCluster{}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "ProxmoxCluster")
				os.Exit(1)
			}
			if err := controller.AddMachineControllerToManager(ctx, mgr, &infrastructurev1alpha1.ProxmoxMachine{}); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "ProxmoxMachine")
				os.Exit(1)
			}
			if err := controller.AddVMControllerToManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "ProxmoxVM")
				os.Exit(1)
			}
			if err := controller.AddProxmoxClusterIdentityControllerToManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "ProxmoxClusterIdentity")
				os.Exit(1)
			}
			if err := controller.AddProxmoxDeploymentZoneControllerToManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "ProxmoxDeploymentZone")
				os.Exit(1)
			}
		}
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
}
