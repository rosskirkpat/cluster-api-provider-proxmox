package manager

import "time"

const (
	defaultPrefix = "cappx-"

	// DefaultWebhookServiceContainerPort is the default value for the eponymous
	// manager option.
	DefaultWebhookServiceContainerPort = 0

	// DefaultSyncPeriod is the default value for the eponymous
	// manager option.
	DefaultSyncPeriod = time.Minute * 10

	// DefaultPodName is the default value for the eponymous manager option.
	DefaultPodName = defaultPrefix + "controller-manager"

	DefaultPodNamespace = defaultPrefix + "system"

	// DefaultLeaderElectionID is the default value for the eponymous manager option.
	DefaultLeaderElectionID = DefaultPodName + "-runtime"
)
