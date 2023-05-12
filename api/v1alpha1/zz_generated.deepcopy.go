//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIEndpoint) DeepCopyInto(out *APIEndpoint) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIEndpoint.
func (in *APIEndpoint) DeepCopy() *APIEndpoint {
	if in == nil {
		return nil
	}
	out := new(APIEndpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AllowedNamespaces) DeepCopyInto(out *AllowedNamespaces) {
	*out = *in
	in.Selector.DeepCopyInto(&out.Selector)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AllowedNamespaces.
func (in *AllowedNamespaces) DeepCopy() *AllowedNamespaces {
	if in == nil {
		return nil
	}
	out := new(AllowedNamespaces)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DHCPOverrides) DeepCopyInto(out *DHCPOverrides) {
	*out = *in
	if in.Hostname != nil {
		in, out := &in.Hostname, &out.Hostname
		*out = new(string)
		**out = **in
	}
	if in.RouteMetric != nil {
		in, out := &in.RouteMetric, &out.RouteMetric
		*out = new(int)
		**out = **in
	}
	if in.SendHostname != nil {
		in, out := &in.SendHostname, &out.SendHostname
		*out = new(bool)
		**out = **in
	}
	if in.UseDNS != nil {
		in, out := &in.UseDNS, &out.UseDNS
		*out = new(bool)
		**out = **in
	}
	if in.UseDomains != nil {
		in, out := &in.UseDomains, &out.UseDomains
		*out = new(string)
		**out = **in
	}
	if in.UseHostname != nil {
		in, out := &in.UseHostname, &out.UseHostname
		*out = new(bool)
		**out = **in
	}
	if in.UseMTU != nil {
		in, out := &in.UseMTU, &out.UseMTU
		*out = new(bool)
		**out = **in
	}
	if in.UseNTP != nil {
		in, out := &in.UseNTP, &out.UseNTP
		*out = new(bool)
		**out = **in
	}
	if in.UseRoutes != nil {
		in, out := &in.UseRoutes, &out.UseRoutes
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DHCPOverrides.
func (in *DHCPOverrides) DeepCopy() *DHCPOverrides {
	if in == nil {
		return nil
	}
	out := new(DHCPOverrides)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkDeviceSpec) DeepCopyInto(out *NetworkDeviceSpec) {
	*out = *in
	if in.IPAddrs != nil {
		in, out := &in.IPAddrs, &out.IPAddrs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.MTU != nil {
		in, out := &in.MTU, &out.MTU
		*out = new(int64)
		**out = **in
	}
	if in.Nameservers != nil {
		in, out := &in.Nameservers, &out.Nameservers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Routes != nil {
		in, out := &in.Routes, &out.Routes
		*out = make([]NetworkRouteSpec, len(*in))
		copy(*out, *in)
	}
	if in.SearchDomains != nil {
		in, out := &in.SearchDomains, &out.SearchDomains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AddressesFromPools != nil {
		in, out := &in.AddressesFromPools, &out.AddressesFromPools
		*out = make([]corev1.TypedLocalObjectReference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DHCP4Overrides != nil {
		in, out := &in.DHCP4Overrides, &out.DHCP4Overrides
		*out = new(DHCPOverrides)
		(*in).DeepCopyInto(*out)
	}
	if in.DHCP6Overrides != nil {
		in, out := &in.DHCP6Overrides, &out.DHCP6Overrides
		*out = new(DHCPOverrides)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkDeviceSpec.
func (in *NetworkDeviceSpec) DeepCopy() *NetworkDeviceSpec {
	if in == nil {
		return nil
	}
	out := new(NetworkDeviceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkRouteSpec) DeepCopyInto(out *NetworkRouteSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkRouteSpec.
func (in *NetworkRouteSpec) DeepCopy() *NetworkRouteSpec {
	if in == nil {
		return nil
	}
	out := new(NetworkRouteSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkSpec) DeepCopyInto(out *NetworkSpec) {
	*out = *in
	if in.Devices != nil {
		in, out := &in.Devices, &out.Devices
		*out = make([]NetworkDeviceSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Routes != nil {
		in, out := &in.Routes, &out.Routes
		*out = make([]NetworkRouteSpec, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkSpec.
func (in *NetworkSpec) DeepCopy() *NetworkSpec {
	if in == nil {
		return nil
	}
	out := new(NetworkSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkStatus) DeepCopyInto(out *NetworkStatus) {
	*out = *in
	if in.IPAddrs != nil {
		in, out := &in.IPAddrs, &out.IPAddrs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkStatus.
func (in *NetworkStatus) DeepCopy() *NetworkStatus {
	if in == nil {
		return nil
	}
	out := new(NetworkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PCIDeviceSpec) DeepCopyInto(out *PCIDeviceSpec) {
	*out = *in
	if in.DeviceID != nil {
		in, out := &in.DeviceID, &out.DeviceID
		*out = new(int32)
		**out = **in
	}
	if in.VendorID != nil {
		in, out := &in.VendorID, &out.VendorID
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PCIDeviceSpec.
func (in *PCIDeviceSpec) DeepCopy() *PCIDeviceSpec {
	if in == nil {
		return nil
	}
	out := new(PCIDeviceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxCluster) DeepCopyInto(out *ProxmoxCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxCluster.
func (in *ProxmoxCluster) DeepCopy() *ProxmoxCluster {
	if in == nil {
		return nil
	}
	out := new(ProxmoxCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProxmoxCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterIdentity) DeepCopyInto(out *ProxmoxClusterIdentity) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterIdentity.
func (in *ProxmoxClusterIdentity) DeepCopy() *ProxmoxClusterIdentity {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterIdentity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProxmoxClusterIdentity) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterIdentityList) DeepCopyInto(out *ProxmoxClusterIdentityList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProxmoxClusterIdentity, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterIdentityList.
func (in *ProxmoxClusterIdentityList) DeepCopy() *ProxmoxClusterIdentityList {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterIdentityList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProxmoxClusterIdentityList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterIdentitySpec) DeepCopyInto(out *ProxmoxClusterIdentitySpec) {
	*out = *in
	if in.AllowedNamespaces != nil {
		in, out := &in.AllowedNamespaces, &out.AllowedNamespaces
		*out = new(AllowedNamespaces)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterIdentitySpec.
func (in *ProxmoxClusterIdentitySpec) DeepCopy() *ProxmoxClusterIdentitySpec {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterIdentitySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterIdentityStatus) DeepCopyInto(out *ProxmoxClusterIdentityStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterIdentityStatus.
func (in *ProxmoxClusterIdentityStatus) DeepCopy() *ProxmoxClusterIdentityStatus {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterIdentityStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterList) DeepCopyInto(out *ProxmoxClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProxmoxCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterList.
func (in *ProxmoxClusterList) DeepCopy() *ProxmoxClusterList {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProxmoxClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterSpec) DeepCopyInto(out *ProxmoxClusterSpec) {
	*out = *in
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	if in.IdentityRef != nil {
		in, out := &in.IdentityRef, &out.IdentityRef
		*out = new(ProxmoxIdentityReference)
		**out = **in
	}
	if in.FailureDomainSelector != nil {
		in, out := &in.FailureDomainSelector, &out.FailureDomainSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterSpec.
func (in *ProxmoxClusterSpec) DeepCopy() *ProxmoxClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxClusterStatus) DeepCopyInto(out *ProxmoxClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make(v1beta1.FailureDomains, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxClusterStatus.
func (in *ProxmoxClusterStatus) DeepCopy() *ProxmoxClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ProxmoxClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxIdentityReference) DeepCopyInto(out *ProxmoxIdentityReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxIdentityReference.
func (in *ProxmoxIdentityReference) DeepCopy() *ProxmoxIdentityReference {
	if in == nil {
		return nil
	}
	out := new(ProxmoxIdentityReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxMachine) DeepCopyInto(out *ProxmoxMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxMachine.
func (in *ProxmoxMachine) DeepCopy() *ProxmoxMachine {
	if in == nil {
		return nil
	}
	out := new(ProxmoxMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProxmoxMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxMachineList) DeepCopyInto(out *ProxmoxMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProxmoxMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxMachineList.
func (in *ProxmoxMachineList) DeepCopy() *ProxmoxMachineList {
	if in == nil {
		return nil
	}
	out := new(ProxmoxMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProxmoxMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxMachineSpec) DeepCopyInto(out *ProxmoxMachineSpec) {
	*out = *in
	in.VirtualMachineCloneSpec.DeepCopyInto(&out.VirtualMachineCloneSpec)
	if in.ProviderID != nil {
		in, out := &in.ProviderID, &out.ProviderID
		*out = new(string)
		**out = **in
	}
	if in.FailureDomain != nil {
		in, out := &in.FailureDomain, &out.FailureDomain
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxMachineSpec.
func (in *ProxmoxMachineSpec) DeepCopy() *ProxmoxMachineSpec {
	if in == nil {
		return nil
	}
	out := new(ProxmoxMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxMachineStatus) DeepCopyInto(out *ProxmoxMachineStatus) {
	*out = *in
	if in.Addresses != nil {
		in, out := &in.Addresses, &out.Addresses
		*out = make([]v1beta1.MachineAddress, len(*in))
		copy(*out, *in)
	}
	if in.Network != nil {
		in, out := &in.Network, &out.Network
		*out = make([]NetworkStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.MachineStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxMachineStatus.
func (in *ProxmoxMachineStatus) DeepCopy() *ProxmoxMachineStatus {
	if in == nil {
		return nil
	}
	out := new(ProxmoxMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProxmoxMachineTemplateResource) DeepCopyInto(out *ProxmoxMachineTemplateResource) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProxmoxMachineTemplateResource.
func (in *ProxmoxMachineTemplateResource) DeepCopy() *ProxmoxMachineTemplateResource {
	if in == nil {
		return nil
	}
	out := new(ProxmoxMachineTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SSHUser) DeepCopyInto(out *SSHUser) {
	*out = *in
	if in.AuthorizedKeys != nil {
		in, out := &in.AuthorizedKeys, &out.AuthorizedKeys
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SSHUser.
func (in *SSHUser) DeepCopy() *SSHUser {
	if in == nil {
		return nil
	}
	out := new(SSHUser)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualMachine) DeepCopyInto(out *VirtualMachine) {
	*out = *in
	if in.Network != nil {
		in, out := &in.Network, &out.Network
		*out = make([]NetworkStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualMachine.
func (in *VirtualMachine) DeepCopy() *VirtualMachine {
	if in == nil {
		return nil
	}
	out := new(VirtualMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VirtualMachineCloneSpec) DeepCopyInto(out *VirtualMachineCloneSpec) {
	*out = *in
	in.Network.DeepCopyInto(&out.Network)
	if in.AdditionalDisksGiB != nil {
		in, out := &in.AdditionalDisksGiB, &out.AdditionalDisksGiB
		*out = make([]int32, len(*in))
		copy(*out, *in)
	}
	if in.TagIDs != nil {
		in, out := &in.TagIDs, &out.TagIDs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PciDevices != nil {
		in, out := &in.PciDevices, &out.PciDevices
		*out = make([]PCIDeviceSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualMachineCloneSpec.
func (in *VirtualMachineCloneSpec) DeepCopy() *VirtualMachineCloneSpec {
	if in == nil {
		return nil
	}
	out := new(VirtualMachineCloneSpec)
	in.DeepCopyInto(out)
	return out
}
