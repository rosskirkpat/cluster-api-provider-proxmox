package util

import (
	"fmt"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// SanitizeHostInfoLabel ensures that the Proxmox host information passed as a parameter confirms to
// the label value constraints documented at
// https://k8s.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
//
// The expected inputs for the object are IP addresses or FQDNs of the Proxmox hosts.
func SanitizeHostInfoLabel(info string) string {
	updatedInfo := stripZoneInfo(info)
	ip := net.ParseIP(updatedInfo)
	if ip != nil {
		if ipv4 := ip.To4(); ipv4 == nil {
			// In case of an IPv6 address, replace `:` with the acceptable `-` character.
			// The size for the string would never exceed 52 (8 * 4 (address) + 7 (dashes) + 13 (suffix) = 52) characters.
			return fmt.Sprintf("%s.ipv6-literal", strings.ReplaceAll(updatedInfo, ":", "-"))
		}
		return updatedInfo
	}
	return truncateLabelLength(info)
}

// stripZoneInfo removes the zone info from an IPv6 address.
// This might not be exactly relevant since zone is used for link-local addresses and
// would not be meaningful outside the host.
func stripZoneInfo(info string) string {
	idx := strings.LastIndex(info, "%")
	if idx == -1 {
		return info
	}
	return info[:idx]
}

func truncateLabelLength(inputURL string) string {
	if len(inputURL) <= validation.LabelValueMaxLength {
		return inputURL
	}

	for {
		pos := strings.LastIndex(inputURL, ".")
		if pos == -1 {
			return inputURL[:validation.LabelValueMaxLength]
		}
		inputURL = inputURL[0:pos]
		if len(inputURL) <= validation.LabelValueMaxLength {
			break
		}
	}
	return inputURL
}
