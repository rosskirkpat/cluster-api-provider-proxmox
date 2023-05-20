package services

import (
	"fmt"
	"github.com/pkg/errors"
	"net/netip"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"strings"

	infrav1 "github.com/rosskirkpat/cluster-api-provider-proxmox/api/v1alpha1"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/context"
	"github.com/rosskirkpat/cluster-api-provider-proxmox/pkg/util"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var ErrWaitingForIPAddr = errors.New("waiting for IP address claims to be bound")

// prefixesAsStrings converts []netip.Prefix to []string.
func prefixesAsStrings(prefixes []netip.Prefix) []string {
	prefixSrings := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		prefixSrings = append(prefixSrings, prefix.String())
	}
	return prefixSrings
}

// parseAddressWithPrefix converts a *ipamv1.IPAddress to a string, e.g. '10.0.0.1/24'.
func parseAddressWithPrefix(ipamAddress *ipamv1.IPAddress) (netip.Prefix, error) {
	addressWithPrefix := fmt.Sprintf("%s/%d", ipamAddress.Spec.Address, ipamAddress.Spec.Prefix)
	parsedPrefix, err := netip.ParsePrefix(addressWithPrefix)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("IPAddress %s/%s has invalid ip address: %q",
			ipamAddress.Namespace,
			ipamAddress.Name,
			addressWithPrefix,
		)
	}

	return parsedPrefix, nil
}

// parseGateway parses the gateway address on an ipamv1.IPAddress and ensures it
// does not conflict with the gateway addresses parsed from other
// ipamv1.IPAddresses on the current device. Gateway addresses must be the same
// family as the address on the ipamv1.IPAddress. Gateway addresses of one
// family must match the other addresses of the same family. IPv4 Gateways are
// required, but IPv6 gateways are not.
func parseGateway(ipamAddress *ipamv1.IPAddress, addressWithPrefix netip.Prefix, ipamDeviceConfig ipamDeviceConfig) (*netip.Addr, error) {
	if ipamAddress.Spec.Gateway == "" && addressWithPrefix.Addr().Is6() {
		return nil, nil
	}

	gatewayAddr, err := netip.ParseAddr(ipamAddress.Spec.Gateway)
	if err != nil {
		return nil, fmt.Errorf("IPAddress %s/%s has invalid gateway: %q",
			ipamAddress.Namespace,
			ipamAddress.Name,
			ipamAddress.Spec.Gateway,
		)
	}

	if addressWithPrefix.Addr().Is4() != gatewayAddr.Is4() {
		return nil, fmt.Errorf("IPAddress %s/%s has mismatched gateway and address IP families",
			ipamAddress.Namespace,
			ipamAddress.Name,
		)
	}

	if gatewayAddr.Is4() {
		if areGatewaysMismatched(ipamDeviceConfig.NetworkSpecGateway4, ipamAddress.Spec.Gateway) {
			return nil, fmt.Errorf("the IPv4 Gateway for IPAddress %s does not match the Gateway4 already configured on device (index %d)",
				ipamAddress.Name,
				ipamDeviceConfig.DeviceIndex,
			)
		}
		if areGatewaysMismatched(ipamDeviceConfig.IPAMConfigGateway4, ipamAddress.Spec.Gateway) {
			return nil, fmt.Errorf("the IPv4 IPAddresses assigned to the same device (index %d) do not have the same gateway",
				ipamDeviceConfig.DeviceIndex,
			)
		}
	} else {
		if areGatewaysMismatched(ipamDeviceConfig.NetworkSpecGateway6, ipamAddress.Spec.Gateway) {
			return nil, fmt.Errorf("the IPv6 Gateway for IPAddress %s does not match the Gateway6 already configured on device (index %d)",
				ipamAddress.Name,
				ipamDeviceConfig.DeviceIndex,
			)
		}
		if areGatewaysMismatched(ipamDeviceConfig.IPAMConfigGateway6, ipamAddress.Spec.Gateway) {
			return nil, fmt.Errorf("the IPv6 IPAddresses assigned to the same device (index %d) do not have the same gateway",
				ipamDeviceConfig.DeviceIndex,
			)
		}
	}

	return &gatewayAddr, nil
}

// areGatewaysMismatched checks that a gateway for a device is equal to an
// IPAddresses gateway. We can assume that IPAddresses will always have
// gateways so we do not need to check for empty string. It is possible to
// configure a device and not a gateway, we don't want to fail in that case.
func areGatewaysMismatched(deviceGateway, ipAddressGateway string) bool {
	return deviceGateway != "" && deviceGateway != ipAddressGateway
}

// ipamDeviceConfig aids and holds state for the process
// of parsing IPAM addresses for a given device.
type ipamDeviceConfig struct {
	DeviceIndex         int
	IPAMAddresses       []*ipamv1.IPAddress
	MACAddress          string
	NetworkSpecGateway4 string
	IPAMConfigGateway4  string
	NetworkSpecGateway6 string
	IPAMConfigGateway6  string
}

func BuildState(ctx context.VMContext, networkStatus []infrav1.NetworkStatus) (map[string]infrav1.NetworkDeviceSpec, error) {
	state := map[string]infrav1.NetworkDeviceSpec{}

	ipamDeviceConfigs, err := buildIPAMDeviceConfigs(ctx, networkStatus)
	if err != nil {
		return state, err
	}

	var errs []error
	for _, ipamDeviceConfig := range ipamDeviceConfigs {
		var addressWithPrefixes []netip.Prefix
		for _, ipamAddress := range ipamDeviceConfig.IPAMAddresses {
			addressWithPrefix, err := parseAddressWithPrefix(ipamAddress)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if slices.Contains(addressWithPrefixes, addressWithPrefix) {
				errs = append(errs,
					fmt.Errorf("IPAddress %s/%s is a duplicate of another address: %q",
						ipamAddress.Namespace,
						ipamAddress.Name,
						addressWithPrefix))
				continue
			}

			gatewayAddr, err := parseGateway(ipamAddress, addressWithPrefix, ipamDeviceConfig)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if gatewayAddr != nil {
				if gatewayAddr.Is4() {
					ipamDeviceConfig.IPAMConfigGateway4 = ipamAddress.Spec.Gateway
				} else {
					ipamDeviceConfig.IPAMConfigGateway6 = ipamAddress.Spec.Gateway
				}
			}

			addressWithPrefixes = append(addressWithPrefixes, addressWithPrefix)
		}

		if len(addressWithPrefixes) > 0 {
			state[ipamDeviceConfig.MACAddress] = infrav1.NetworkDeviceSpec{
				IPAddrs:  prefixesAsStrings(addressWithPrefixes),
				Gateway4: ipamDeviceConfig.IPAMConfigGateway4,
				Gateway6: ipamDeviceConfig.IPAMConfigGateway6,
			}
		}
	}

	if len(errs) > 0 {
		var msgs []string
		for _, err := range errs {
			msgs = append(msgs, err.Error())
		}
		msg := strings.Join(msgs, "\n")
		return state, errors.New(msg)
	}
	return state, nil
}

// buildIPAMDeviceConfigs checks that all the IPAddressClaims have been satisfied.
// If each IPAddressClaim has an associated IPAddress, a slice of ipamDeviceConfig
// is returned, one for each device with addressesFromPools.
// If any of the IPAddressClaims do not have an associated IPAddress yet,
// a custom error is returned.
func buildIPAMDeviceConfigs(ctx context.VMContext, networkStatus []infrav1.NetworkStatus) ([]ipamDeviceConfig, error) {
	boundClaims, totalClaims := 0, 0
	ipamDeviceConfigs := []ipamDeviceConfig{}
	for devIdx, networkSpecDevice := range ctx.ProxmoxVM.Spec.Network.Devices {
		if len(networkStatus) == 0 ||
			len(networkStatus) <= devIdx ||
			networkStatus[devIdx].MACAddr == "" {
			return ipamDeviceConfigs, errors.New("waiting for devices to have MAC address set")
		}

		ipamDeviceConfig := ipamDeviceConfig{
			IPAMAddresses:       []*ipamv1.IPAddress{},
			MACAddress:          networkStatus[devIdx].MACAddr,
			NetworkSpecGateway4: networkSpecDevice.Gateway4,
			NetworkSpecGateway6: networkSpecDevice.Gateway6,
			DeviceIndex:         devIdx,
		}

		for poolRefIdx := range networkSpecDevice.AddressesFromPools {
			totalClaims++
			ipAddrClaimName := util.IPAddressClaimName(ctx.ProxmoxVM.Name, ipamDeviceConfig.DeviceIndex, poolRefIdx)
			ipAddrClaim, err := getIPAddrClaim(ctx, ipAddrClaimName)
			if err != nil {
				ctx.Logger.Error(err, "error fetching IPAddressClaim", "name", ipAddrClaimName)
				if apierrors.IsNotFound(err) {
					// it would be odd for this to occur, a findorcreate just happened in a previous step
					continue
				}
				return nil, err
			}

			ctx.Logger.V(5).Info("fetched IPAddressClaim", "name", ipAddrClaimName, "namespace", ctx.ProxmoxVM.Namespace)
			ipAddrName := ipAddrClaim.Status.AddressRef.Name
			if ipAddrName == "" {
				ctx.Logger.V(5).Info("IPAddress not yet bound to IPAddressClaim", "name", ipAddrClaimName, "namespace", ctx.ProxmoxVM.Namespace)
				continue
			}

			ipAddr := &ipamv1.IPAddress{}
			ipAddrKey := apitypes.NamespacedName{
				Namespace: ctx.ProxmoxVM.Namespace,
				Name:      ipAddrName,
			}
			if err := ctx.Client.Get(ctx, ipAddrKey, ipAddr); err != nil {
				// because the ref was set on the claim, it is expected this error should not occur
				return nil, err
			}
			ipamDeviceConfig.IPAMAddresses = append(ipamDeviceConfig.IPAMAddresses, ipAddr)
			boundClaims++
		}
		ipamDeviceConfigs = append(ipamDeviceConfigs, ipamDeviceConfig)
	}
	if boundClaims < totalClaims {
		ctx.Logger.Info("waiting for ip address claims to be bound",
			"total claims", totalClaims,
			"claims bound", boundClaims)
		return nil, ErrWaitingForIPAddr
	}
	return ipamDeviceConfigs, nil
}

// getIPAddrClaim fetches an IPAddressClaim from the api with the given name.
func getIPAddrClaim(ctx context.VMContext, ipAddrClaimName string) (*ipamv1.IPAddressClaim, error) {
	ipAddrClaim := &ipamv1.IPAddressClaim{}
	ipAddrClaimKey := apitypes.NamespacedName{
		Namespace: ctx.ProxmoxVM.Namespace,
		Name:      ipAddrClaimName,
	}

	ctx.Logger.V(5).Info("fetching IPAddressClaim", "name", ipAddrClaimKey.String())
	if err := ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim); err != nil {
		return nil, err
	}
	return ipAddrClaim, nil
}
