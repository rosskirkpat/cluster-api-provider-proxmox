package context

import (
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MachineContext interface {
	String() string
	Patch() error
	GetLogger() logr.Logger
	GetProxmoxMachine() ProxmoxMachine
	GetObjectMeta() v1.ObjectMeta
	GetCluster() *clusterv1.Cluster
	GetMachine() *clusterv1.Machine
	SetBaseMachineContext(base *BaseMachineContext)
}

type ProxmoxMachine interface {
	client.Object
	conditions.Setter
}
