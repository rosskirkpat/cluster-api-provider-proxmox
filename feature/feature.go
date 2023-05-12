package feature

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// Every cappx-specific feature gate should add method here following this template:
	//
	// // owner: @username
	// // alpha: v1.X
	// MyFeature featuregate.Feature = "MyFeature".

	// NodeAntiAffinity is a feature gate for the NodeAntiAffinity functionality.
	//
	// alpha: v1.0
	NodeAntiAffinity featuregate.Feature = "NodeAntiAffinity"
)

func init() {
	runtime.Must(MutableGates.Add(defaultCAPPXFeatureGates))
}

// defaultCAPPXFeatureGates consists of all known cappx-specific feature keys.
// To add a new feature, define a key for it above and add it here.
var defaultCAPPXFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	// Every feature should be initiated here:
	NodeAntiAffinity: {Default: false, PreRelease: featuregate.Alpha},
}
