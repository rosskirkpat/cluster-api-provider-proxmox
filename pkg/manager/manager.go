package manager

import (
	goctx "context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/record"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Manager is a CAPPX controller manager.
type Manager interface {
	ctrl.Manager

	// GetContext returns the controller manager's context.
	GetContext() *context.ControllerContext
}

// New returns a new CAPPX controller manager.
func New(opts Options) (Manager, error) {
	// Ensure the default options are set.
	opts.defaults()

	_ = clientgoscheme.AddToScheme(opts.Scheme)
	_ = clusterv1.AddToScheme(opts.Scheme)
	_ = v1alpha1.AddToScheme(opts.Scheme)
	_ = controlplanev1.AddToScheme(opts.Scheme)
	_ = bootstrapv1.AddToScheme(opts.Scheme)
	_ = ipamv1.AddToScheme(opts.Scheme)
	// +kubebuilder:scaffold:scheme

	//podName, err := os.Hostname()
	//if err != nil {
	podName := DefaultPodName
	//}

	// Build the controller manager.
	mgr, err := ctrl.NewManager(opts.KubeConfig, opts.Options)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create manager")
	}

	// Build the controller manager context.
	controllerManagerContext := &context.ControllerContext{
		Context:                 goctx.Background(),
		WatchNamespace:          opts.Namespace,
		Namespace:               opts.PodNamespace,
		Name:                    opts.PodName,
		LeaderElectionID:        opts.LeaderElectionID,
		LeaderElectionNamespace: opts.LeaderElectionNamespace,
		MaxConcurrentReconciles: opts.MaxConcurrentReconciles,
		Client:                  mgr.GetClient(),
		Logger:                  opts.Logger.WithName(opts.PodName),
		Recorder:                record.New(mgr.GetEventRecorderFor(fmt.Sprintf("%s/%s", opts.PodNamespace, podName))),
		Scheme:                  opts.Scheme,
		Username:                opts.Username,
		Password:                opts.Password,
		EnableKeepAlive:         opts.EnableKeepAlive,
		KeepAliveDuration:       opts.KeepAliveDuration,
	}

	// Add the requested items to the manager.
	if err := opts.AddToManager(controllerManagerContext, mgr); err != nil {
		return nil, errors.Wrap(err, "failed to add resources to the manager")
	}

	// +kubebuilder:scaffold:builder

	return &manager{
		Manager: mgr,
		ctx:     controllerManagerContext,
	}, nil
}

type manager struct {
	ctrl.Manager
	ctx *context.ControllerContext
}

func (m *manager) GetContext() *context.ControllerContext {
	return m.ctx
}

func UpdateCredentials(opts *Options) {
	opts.readAndSetCredentials()
}

// InitializeWatch adds a filesystem watcher for the cappx credentials file
// In case of any update to the credentials file, the new credentials are passed to the cappx manager context.
func InitializeWatch(ctx *context.ControllerContext, managerOpts *Options) (watch *fsnotify.Watcher, err error) {
	cappxCredentialsFile := managerOpts.CredentialsFile
	updateEventCh := make(chan bool)
	watch, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create new Watcher for %s", cappxCredentialsFile))
	}
	if err = watch.Add(cappxCredentialsFile); err != nil {
		return nil, errors.Wrap(err, "received error on CAPPX credential watcher")
	}
	go func() {
		for {
			select {
			case err := <-watch.Errors:
				ctx.Logger.Error(err, "received error on CAPPX credential watcher")
			case event := <-watch.Events:
				ctx.Logger.Info(fmt.Sprintf("received event %v on the credential file %s", event, cappxCredentialsFile))
				updateEventCh <- true
			}
		}
	}()

	go func() {
		for range updateEventCh {
			UpdateCredentials(managerOpts)
		}
	}()

	return watch, err
}
