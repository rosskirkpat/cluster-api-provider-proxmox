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
	goctx "context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/certs"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/services"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"reflect"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"

	"github.com/fsnotify/fsnotify"
	infrastructurev1alpha1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	controller "github.com/rosskirkpat/cluster-api-provider-proxmox/controllers"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/constants"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/manager"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/session"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	//+kubebuilder:scaffold:imports
)

var (
	configFile string

	logOptions       = logs.NewOptions()
	scheme           = runtime.NewScheme()
	setupLog         = ctrl.Log.WithName("setup")
	ManagerNamespace = "cappx-system"
	ManagerName      = "cappx-manager"

	managerOpts     manager.Options
	syncPeriod      time.Duration
	profilerAddress string

	//tlsOptions = flags.TLSOptions{}

	defaultProfilerAddr      = os.Getenv("PROFILER_ADDR")
	defaultSyncPeriod        = manager.DefaultSyncPeriod
	defaultLeaderElectionID  = manager.DefaultLeaderElectionID
	defaultPodName           = manager.DefaultPodName
	defaultWebhookPort       = manager.DefaultWebhookServiceContainerPort
	defaultEnableKeepAlive   = constants.DefaultEnableKeepAlive
	defaultKeepAliveDuration = constants.DefaultKeepAliveDuration
)

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(logOptions, fs)
	flag.StringVar(
		&configFile,
		"config",
		"controller_manager_config.yaml",
		"The controller manager config file")
	flag.StringVar(
		&managerOpts.MetricsBindAddress,
		"metrics-bind-addr",
		"localhost:8080",
		"The address the metric endpoint binds to.")
	flag.BoolVar(
		&managerOpts.LeaderElection,
		"leader-elect",
		true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(
		&managerOpts.LeaderElectionID,
		"leader-election-id",
		defaultLeaderElectionID,
		"Name of the config map to use as the locking resource when configuring leader election.")
	flag.StringVar(
		&managerOpts.Namespace,
		"namespace",
		ManagerNamespace,
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	flag.StringVar(
		&profilerAddress,
		"profiler-address",
		defaultProfilerAddr,
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")
	flag.DurationVar(
		&syncPeriod,
		"sync-period",
		defaultSyncPeriod,
		"The interval at which cluster-api objects are synchronized")
	flag.IntVar(
		&managerOpts.MaxConcurrentReconciles,
		"max-concurrent-reconciles",
		10,
		"The maximum number of allowed, concurrent reconciles.")
	flag.StringVar(
		&managerOpts.PodName,
		"pod-name",
		defaultPodName,
		"The name of the pod running the controller manager.")
	flag.IntVar(
		&managerOpts.Port,
		"webhook-port",
		defaultWebhookPort,
		"Webhook Server port (set to 0 to disable)")
	flag.StringVar(
		&managerOpts.HealthProbeBindAddress,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)
	flag.StringVar(
		&managerOpts.CredentialsFile,
		"credentials-file",
		"/etc/cappx/credentials.yaml",
		"path to CAPPX's credentials file",
	)
	flag.BoolVar(
		&managerOpts.EnableKeepAlive,
		"enable-keep-alive",
		defaultEnableKeepAlive,
		"feature to enable keep alive handler in proxmox sessions. This functionality is enabled by default.")
	flag.DurationVar(
		&managerOpts.KeepAliveDuration,
		"keep-alive-duration",
		defaultKeepAliveDuration,
		"idle time interval(minutes) in between send() requests in keepalive handler",
	)
	//flags.AddTLSOptions(fs, &tlsOptions)

}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(ipamv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {

	rand.Seed(time.Now().UnixNano())

	InitFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	if err := pflag.CommandLine.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set log level: %v")
		os.Exit(1)
	}
	pflag.Parse()

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// klog.Background will automatically use the right logger.
	//ctrl.SetLogger(klog.Background())
	//managerOpts.LeaderElectionNamespace = ManagerNamespace

	if managerOpts.Namespace != "" {
		setupLog.Info(
			"Watching objects only in namespace for reconciliation",
			"namespace", managerOpts.Namespace)
	}

	if profilerAddress != "" {
		setupLog.Info(
			"Profiler listening for requests",
			"profiler-address", profilerAddress)
		go runProfiler(profilerAddress)
	}
	//
	//flag.StringVar(&configFile, "config", "",
	//	"The controller will load its initial configuration from this file. "+
	//		"Omit this flag to use the default configuration values. "+
	//		"Command-line flags override configuration from this file.")
	//
	opts := zap.Options{
		Development: true,
	}
	//opts.BindFlags(flag.CommandLine)
	//flag.Parse()
	//
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
	//
	//tlsOptionOverrides, err := flags.GetTLSOptionOverrideFuncs(tlsOptions)
	//if err != nil {
	//	setupLog.Error(err, "unable to add TLS settings to the webhook server")
	//	os.Exit(1)
	//}
	//managerOpts.TLSOpts = tlsOptionOverrides

	//mgr, err := manager.New(managerOpts)
	//if err != nil {
	//	setupLog.Error(err, "unable to create new manager")
	//	os.Exit(1)
	//
	//}
	//

	certPem, keyPem, _ := certs.NewSelfSignedCert()
	cert, _ := tls.X509KeyPair(certPem, keyPem)

	var tlsOptions []func(config *tls.Config)
	tlsOptions = append(tlsOptions, func(cfg *tls.Config) {
		cfg.Certificates = []tls.Certificate{cert}
	})
	options.TLSOpts = tlsOptions
	thirty := 30 * time.Second
	//ten := 10 * time.Second
	options.LeaseDuration = &thirty
	//options.RenewDeadline = &thirty
	//options.RetryPeriod = &ten

	temp, err := os.MkdirTemp("", "certs")
	if err != nil {
		return
	}
	_ = os.WriteFile(path.Join(temp, "tls.key"), keyPem, 0444)
	_ = os.WriteFile(path.Join(temp, "tls.crt"), certPem, 0444)
	options.CertDir = temp

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	//
	//options.WebhookServer = mgr.GetWebhookServer()
	//ws := mgr.GetWebhookServer()
	//ws.TLSOpts = tlsOptions

	ctx := &context.ControllerContext{
		Context:   goctx.Background(),
		Namespace: ManagerNamespace,
		Name:      ManagerName,
		Logger:    mgr.GetLogger(),
		Scheme:    mgr.GetScheme(),
	}

	if err = (&controller.ProxmoxClusterReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ControllerContext: ctx,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxCluster")
		os.Exit(1)
	}
	if err = (&controller.ProxmoxMachineReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ControllerContext: ctx,
		VMService:         &services.PimMachineService{},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxMachine")
		os.Exit(1)
	}

	if err = (&controller.ProxmoxVMReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ControllerContext: ctx,
		VMService:         &services.VMService{},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxVM")
		os.Exit(1)
	}
	//if err := controller.AddClusterControllerToManager(ctx, mgr, &infrastructurev1alpha1.ProxmoxCluster{}); err != nil {
	//	setupLog.Error(err, "unable to create controller", "controller", "ProxmoxCluster")
	//	os.Exit(1)
	//}
	//if err := controller.AddMachineControllerToManager(ctx, mgr, &infrastructurev1alpha1.ProxmoxMachine{}); err != nil {
	//	setupLog.Error(err, "unable to create controller", "controller", "ProxmoxMachine")
	//	os.Exit(1)
	//}
	//if err := controller.AddVMControllerToManager(ctx, mgr); err != nil {
	//	setupLog.Error(err, "unable to create controller", "controller", "ProxmoxVM")
	//	os.Exit(1)
	//}

	cluster := &infrastructurev1alpha1.ProxmoxCluster{}
	gvr := infrastructurev1alpha1.GroupVersion.WithResource(reflect.TypeOf(cluster).Elem().Name())
	_, err = mgr.GetRESTMapper().KindFor(gvr)
	if err != nil {
		if meta.IsNoMatchError(err) {
			setupLog.Info(fmt.Sprintf("CRD for %s not loaded, skipping.", gvr.String()))
		} else {
			setupLog.Error(err, fmt.Sprintf("unexpected error while looking for CRD %s", gvr.String()), "crd", "ProxmoxCluster")
			os.Exit(1)
		}
	}

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

	if err := controller.AddProxmoxClusterIdentityControllerToManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxClusterIdentity")
		os.Exit(1)
	}
	if err := controller.AddProxmoxDeploymentZoneControllerToManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProxmoxDeploymentZone")
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

	// initialize notifier for cappx-manager-bootstrap-credentials

	watch, err := manager.InitializeWatch(ctx, &managerOpts)
	if err != nil {
		setupLog.Error(err, "failed to initialize watch on CAPPX credentials file")
		os.Exit(1)
	}
	defer func(watch *fsnotify.Watcher) {
		_ = watch.Close()
	}(watch)
	defer session.Clear()
}

func runProfiler(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		setupLog.Error(err, "problem running profiler server")
	}
}
