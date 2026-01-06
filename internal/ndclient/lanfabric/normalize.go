package lanfabric

import (
	"strings"

	"github.com/banglin/go-nd/internal/ndclient/common"
)

// NormalizeInterfaceName canonicalizes interface names for consistent matching.
// Interface names must be in full NDFC format: "Ethernet" followed by slot/port pattern.
// Supported patterns:
//   - "Ethernet1/1" (standard port)
//   - "Ethernet1/1/1" (breakout port)
//
// This function trims whitespace but does NOT convert short forms.
// NDFC API requires exact format - short forms like "Eth1/3" are not allowed.
func NormalizeInterfaceName(name string) string {
	return strings.TrimSpace(name)
}

// NormalizeInterface converts raw InterfaceData to normalized SwitchPortData
func NormalizeInterface(iface InterfaceData) SwitchPortData {
	return SwitchPortData{
		SerialNumber: iface.SerialNumber,
		Name:         NormalizeInterfaceName(iface.IfName),
		Description:  common.GetString(iface.NvPairs, "DESC"),
		Speed:        common.GetString(iface.NvPairs, "SPEED"),
		MTU:          common.GetString(iface.NvPairs, "MTU"),
		AdminState:   common.GetString(iface.NvPairs, "ADMIN_STATE"),
	}
}

// NormalizeInterfaces converts a slice of raw interfaces to normalized ports
func NormalizeInterfaces(ifaces []InterfaceData) []SwitchPortData {
	ports := make([]SwitchPortData, 0, len(ifaces))
	for _, iface := range ifaces {
		ports = append(ports, NormalizeInterface(iface))
	}
	return ports
}
