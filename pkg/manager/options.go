package manager

import (
	"os"
	"strings"
	"time"

	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/yaml"
)

// AddToManagerFunc is a function that can be optionally specified with
// the manager's Options in order to explicitly decide what controllers and
// webhooks to add to the manager.
type AddToManagerFunc func(*context.ControllerContext, ctrlmgr.Manager) error

// Options describes the options used to create a new CAPPX manager.
type Options struct {
	ctrlmgr.Options

	// EnableKeepAlive is a session feature to enable keep alive handler
	// for better load management on the proxmox api server
	EnableKeepAlive bool

	// MaxConcurrentReconciles the maximum number of allowed, concurrent
	// reconciles.
	//
	// Defaults to the eponymous constant in this package.
	MaxConcurrentReconciles int

	// LeaderElectionNamespace is the namespace in which the pod running the
	// controller maintains a leader election lock
	//
	// Defaults to the eponymous constant in this package.
	PodNamespace string

	// PodName is the name of the pod running the controller manager.
	//
	// Defaults to the eponymous constant in this package.
	PodName string

	// Username is the username for the account used to access remote Proxmox
	// endpoints.
	Username string

	// Password is the password for the account used to access remote Proxmox
	// endpoints.
	Password string

	// KeepAliveDuration is the idle time interval in between send() requests
	// in keepalive handler
	KeepAliveDuration time.Duration

	// CredentialsFile is the file that contains credentials of CAPPX
	CredentialsFile string

	KubeConfig *rest.Config

	// AddToManager is a function that can be optionally specified with
	// the manager's Options in order to explicitly decide what controllers
	// and webhooks to add to the manager.
	AddToManager AddToManagerFunc
}

func (o *Options) defaults() {
	if o.Logger.GetSink() == nil {
		o.Logger = ctrllog.Log
	}

	if o.PodName == "" {
		o.PodName = DefaultPodName
	}

	if o.KubeConfig == nil {
		o.KubeConfig = config.GetConfigOrDie()
	}

	if o.Scheme == nil {
		o.Scheme = runtime.NewScheme()
	}

	if o.Username == "" || o.Password == "" {
		o.readAndSetCredentials()
	}

	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		o.PodNamespace = ns
	} else if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			o.PodNamespace = ns
		}
	} else {
		o.PodNamespace = DefaultPodNamespace
	}
}

func (o *Options) getCredentials() map[string]string {
	file, err := os.ReadFile(o.CredentialsFile)
	if err != nil {
		o.Logger.Error(err, "error opening credentials file")
		return map[string]string{}
	}

	credentials := map[string]string{}
	if err := yaml.Unmarshal(file, &credentials); err != nil {
		o.Logger.Error(err, "error unmarshalling credentials to yaml")
		return map[string]string{}
	}

	return credentials
}

func (o *Options) readAndSetCredentials() {
	credentials := o.getCredentials()
	o.Username = credentials["username"]
	o.Password = credentials["password"]
}
