package context

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/record"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// ControllerContext is the context of a controller.
type ControllerContext struct {
	context.Context

	// Namespace is the namespace in which the resource is located responsible
	// for running the controller manager.
	Namespace string

	// Name is the name of the controller manager.
	Name string

	// LeaderElectionID is the information used to identify the object
	// responsible for synchronizing leader election.
	LeaderElectionID string

	// LeaderElectionNamespace is the namespace in which the LeaderElection
	// object is located.
	LeaderElectionNamespace string

	// WatchNamespace is the namespace the controllers watch for changes. If
	// no value is specified then all namespaces are watched.
	WatchNamespace string

	// Client is the controller manager's client.
	Client client.Client

	// Logger is the controller manager's logger.
	Logger logr.Logger

	// Recorder is used to record events.
	Recorder record.Recorder

	// Scheme is the controller manager's API scheme.
	Scheme *runtime.Scheme

	// MaxConcurrentReconciles is the maximum number of reconcile requests this
	// controller will receive concurrently.
	MaxConcurrentReconciles int

	// Username is the username for the account used to access remote Proxmox
	// endpoints.
	Username string

	// Password is the password for the account used to access remote Proxmox
	// endpoints.
	Password string

	// EnableKeepAlive is a session feature to enable keep alive handler
	// for better load management on Proxmox api server
	EnableKeepAlive bool

	// KeepAliveDuration is the idle time interval in between send() requests
	// in keepalive handler
	KeepAliveDuration time.Duration

	genericEventCache sync.Map
}

// String returns ControllerName.
func (c *ControllerContext) String() string {
	return c.Name
}

// GetGenericEventChannelFor returns a generic event channel for a resource
// specified by the provided GroupVersionKind.
func (c *ControllerContext) GetGenericEventChannelFor(gvk schema.GroupVersionKind) chan event.GenericEvent {
	if val, ok := c.genericEventCache.Load(gvk); ok {
		return val.(chan event.GenericEvent)
	}
	val, _ := c.genericEventCache.LoadOrStore(gvk, make(chan event.GenericEvent))
	return val.(chan event.GenericEvent)
}
