package session

import (
	"fmt"
	"strings"
)

const (
	errString = "Proxmox version cannot be identified"
)

type unidentifiedProxmoxVersion struct {
	version string
}

func (e unidentifiedProxmoxVersion) Error() string {
	return fmt.Sprintf("%s: %s", errString, e.version)
}

func IsUnidentifiedProxmoxVersion(err error) bool {
	return strings.HasPrefix(err.Error(), errString)
}
