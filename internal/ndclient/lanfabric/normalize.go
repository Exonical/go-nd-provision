package lanfabric

import (
	"github.com/banglin/go-nd/internal/ndclient/common"
)

// NormalizeInterface converts raw InterfaceData to normalized SwitchPortData
func NormalizeInterface(iface InterfaceData) SwitchPortData {
	return SwitchPortData{
		SerialNumber: iface.SerialNumber,
		Name:         iface.IfName,
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
